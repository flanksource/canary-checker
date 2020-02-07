package checks

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jasonlvhit/gocron"
	"github.com/jinzhu/copier"
	"golang.org/x/net/context"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dockerImagePullFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_docker_pull_failed",
		Help: "The total number of Docker image pull failed",
	})

	totalLayers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_total_layers",
			Help: "Total layers in docker image",
		},
		[]string{"image"},
	)

	size = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_image_size",
			Help: "size of docker image in MB",
		},
		[]string{"image"},
	)

	imagePullTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_image_pull_time",
			Help:    "Image Pull in Seconds",
			Buckets: []float64{10, 25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
		},
		[]string{"image"},
	)
)

func init() {
	prometheus.MustRegister(dockerImagePullFailed, totalLayers, size, imagePullTime)
}

type DockerInspect struct {
	ID              string    `json:"Id"`
	RepoTags        []string  `json:"RepoTags"`
	RepoDigests     []string  `json:"RepoDigests"`
	Parent          string    `json:"Parent"`
	Comment         string    `json:"Comment"`
	Created         time.Time `json:"Created"`
	Container       string    `json:"Container"`
	ContainerConfig struct {
		Hostname     string      `json:"Hostname"`
		Domainname   string      `json:"Domainname"`
		User         string      `json:"User"`
		AttachStdin  bool        `json:"AttachStdin"`
		AttachStdout bool        `json:"AttachStdout"`
		AttachStderr bool        `json:"AttachStderr"`
		Tty          bool        `json:"Tty"`
		OpenStdin    bool        `json:"OpenStdin"`
		StdinOnce    bool        `json:"StdinOnce"`
		Env          []string    `json:"Env"`
		Cmd          []string    `json:"Cmd"`
		ArgsEscaped  bool        `json:"ArgsEscaped"`
		Image        string      `json:"Image"`
		Volumes      interface{} `json:"Volumes"`
		WorkingDir   string      `json:"WorkingDir"`
		Entrypoint   interface{} `json:"Entrypoint"`
		OnBuild      interface{} `json:"OnBuild"`
		Labels       struct {
		} `json:"Labels"`
	} `json:"ContainerConfig"`
	DockerVersion string `json:"DockerVersion"`
	Author        string `json:"Author"`
	Config        struct {
		Hostname     string      `json:"Hostname"`
		Domainname   string      `json:"Domainname"`
		User         string      `json:"User"`
		AttachStdin  bool        `json:"AttachStdin"`
		AttachStdout bool        `json:"AttachStdout"`
		AttachStderr bool        `json:"AttachStderr"`
		Tty          bool        `json:"Tty"`
		OpenStdin    bool        `json:"OpenStdin"`
		StdinOnce    bool        `json:"StdinOnce"`
		Env          []string    `json:"Env"`
		Cmd          []string    `json:"Cmd"`
		ArgsEscaped  bool        `json:"ArgsEscaped"`
		Image        string      `json:"Image"`
		Volumes      interface{} `json:"Volumes"`
		WorkingDir   string      `json:"WorkingDir"`
		Entrypoint   interface{} `json:"Entrypoint"`
		OnBuild      interface{} `json:"OnBuild"`
		Labels       interface{} `json:"Labels"`
	} `json:"Config"`
	Architecture string `json:"Architecture"`
	Os           string `json:"Os"`
	Size         int    `json:"Size"`
	VirtualSize  int    `json:"VirtualSize"`
	GraphDriver  struct {
		Data struct {
			MergedDir string `json:"MergedDir"`
			UpperDir  string `json:"UpperDir"`
			WorkDir   string `json:"WorkDir"`
		} `json:"Data"`
		Name string `json:"Name"`
	} `json:"GraphDriver"`
	RootFS struct {
		Type   string   `json:"Type"`
		Layers []string `json:"Layers"`
	} `json:"RootFS"`
	Metadata struct {
		LastTagTime time.Time `json:"LastTagTime"`
	} `json:"Metadata"`
}

type DockerPullChecker struct{}

// Type: returns checker type
func (c *DockerPullChecker) Type() string {
	return "dockerpull"
}

// Schedule: Add every check as a cron job, calls MetricProcessor with the set of metrics
func (c *DockerPullChecker) Schedule(config pkg.Config, interval uint64, mp MetricProcessor) {
	for _, conf := range config.DockerPull {
		DockerPullCheck := pkg.DockerPullCheck{}
		if err := copier.Copy(&DockerPullCheck, &conf.DockerPullCheck); err != nil {
			log.Printf("error copying %v", err)
		}
		gocron.Every(interval).Seconds().Do(func() {
			metrics := c.Check(DockerPullCheck)
			mp(metrics)
		})
	}
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPullChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.DockerPull {
		for _, result := range c.Check(conf.DockerPullCheck) {
			checks = append(checks, result)
			fmt.Println(result)
		}
	}
	return checks
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *DockerPullChecker) Check(check pkg.DockerPullCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	ctx := context.Background()
	//cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	for _, image := range check.Images {
		start := time.Now()
		_, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
		elapsed := time.Since(start)
		if err != nil {
			log.Printf("Failed to Pull Docker Image %s", image)
			log.Println(err)
			dockerImagePullFailed.Inc()
			checkResult := &pkg.CheckResult{
				Pass:     false,
				Invalid:  true,
				Endpoint: image,
				Metrics:  []pkg.Metric{},
			}
			result = append(result, checkResult)
			continue
		}

		_, inspect, err := cli.ImageInspectWithRaw(ctx, image)
		if err != nil {
			panic(err)
		}

		dockerInpect := DockerInspect{}
		err = json.Unmarshal(inspect, &dockerInpect)
		if err != nil {
			panic(err)
		}

		layersCount := len(dockerInpect.RootFS.Layers)
		imageSize := float64(dockerInpect.Size / 1024 / 1024)
		pullTime := float64(elapsed.Seconds())

		m := []pkg.Metric{
			{
				Name: "pullTime",
				Type: pkg.HistogramType,
				Labels: map[string]string{
					"image": image,
				},
				Value: float64(pullTime),
			},
			{
				Name: "totalLayers",
				Type: pkg.GaugeType,
				Labels: map[string]string{
					"image": image,
				},
				Value: float64(layersCount),
			},
			{
				Name: "size",
				Type: pkg.GaugeType,
				Labels: map[string]string{
					"image": image,
				},
				Value: float64(imageSize),
			},
		}

		checkResult := &pkg.CheckResult{
			Pass:     true,
			Invalid:  false,
			Duration: int64(pullTime),
			Endpoint: image,
			Metrics:  m,
		}
		result = append(result, checkResult)

		totalLayers.WithLabelValues(image).Set(float64(layersCount))
		size.WithLabelValues(image).Set(float64(imageSize))
		imagePullTime.WithLabelValues(image).Observe(float64(pullTime))
	}
	return result
}
