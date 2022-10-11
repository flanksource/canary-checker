## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/jmeter.svg' style='height: 32px'/> Jmeter

Jmeter check will run jmeter cli against the supplied host
??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| host | Host is the server against which test plan needs to be executed | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **jmx** | Jmx defines tge ConfigMap or Secret reference to get the JMX test plan | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| name | Name of the check | string |  |
| port | Port on which the server is running | int32 |  |
| properties | Properties defines the local Jmeter properties | \[\]string |  |
| responseDuration | ResponseDuration under which the all the test should pass | string |  |
| systemProperties | SystemProperties defines the java system property | \[\]string |  |
