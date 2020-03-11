package checks

import (
	sql "database/sql"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/ghodss/yaml"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

func init() {
	//register metrics here
}

type PostgresChecker struct{}

// Type: returns checker type
func (c *PostgresChecker) Type() string {
	return "postgres"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PostgresChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.Postgres {
		for _, result := range c.Check(conf.PostgresCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *PostgresChecker) Check(check pkg.PostgresCheck) []*pkg.CheckResult {

	var results []*pkg.CheckResult
	var elapsed time.Duration

	start := time.Now()
	db, err := connectWithDriver(check.Driver, check.Connection)
	if err != nil {
		log.Errorf(err.Error())
	}
	defer db.Close()

	//if we have a `result` config we do a simple single result query
	if check.Result != nil {

		queryResult, err := executeSimpleQuery(db, check.Query)
		elapsed = time.Since(start)
		if (err != nil) || (queryResult != *check.Result) {
			checkResult := &pkg.CheckResult{
				Pass:     false,
				Invalid:  false,
				Duration: elapsed.Milliseconds(),
				Endpoint: obfuscateConnectionStringPassword(check.Connection),
				Metrics:  []pkg.Metric{},
			}
			if err != nil {
				log.Errorf(err.Error())
			}
			if queryResult != *check.Result {
				log.Errorf("Query '%s', did not return '%d', but '%d'", check.Query, check.Result, queryResult)
			}
			results = append(results, checkResult)
			return results
		}

		checkResult := &pkg.CheckResult{
			Pass:     true,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Endpoint: obfuscateConnectionStringPassword(check.Connection),
			Metrics:  []pkg.Metric{},
		}
		results = append(results, checkResult)
		log.Debugf("Duration %f", float64(elapsed.Milliseconds()))
		return results
	}

	// //if we have a `results` config we do a simple single result query
	// if len(check.Results.Values) >0 {
	// }
	return results

}

// Connects to a db using the specified `driver` and `connectionstring`

func connectWithDriver(driver string, connectionSting string) (*sql.DB, error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return db, nil
}

// Performs the test query given in `query`.
// Gives the single row test query result as result.
func rest(db *sql.DB, driver string, connectionSting string, query string) (int, error) {
	defer db.Close()

	var resultValue int
	err := db.QueryRow(query).Scan(&resultValue)
	if err != nil {
		log.Error(err.Error())
		return 0, err
	}
	log.Debugf("Connection test query result of %d", resultValue)

	return resultValue, nil
}

func executeSimpleQuery(db *sql.DB, query string) (int, error) {
	var resultValue int
	var err error = db.QueryRow(query).Scan(&resultValue)
	if err != nil {
		log.Error(err.Error())
		return 0, err
	}

	log.Debugf("Connection test query result of %d", resultValue)
	return resultValue, nil
}

func executeComplexQuery(db *sql.DB, query string) ([]pkg.PostgresResults, error) {

	// an array of JSON objects
	// the map key is the field name
	var objects []map[string]interface{}

	var results []pkg.PostgresResults = make([]pkg.PostgresResults, 0)

	rows, err := db.Query(query)
	if err != nil {
		log.Error(err.Error())
		return []pkg.PostgresResults{}, err
	}
	log.Debugf("%v", rows)

	for rows.Next() {
		// figure out what columns were returned
		// the column names will be the JSON object field keys
		columns, err := rows.ColumnTypes()
		if err != nil {
			return []pkg.PostgresResults{}, err
		}

		// Scan needs an array of pointers to the values it is setting
		// This creates the object and sets the values correctly
		values := make([]interface{}, len(columns))
		object := map[string]interface{}{}
		for i, column := range columns {
			object[column.Name()] = reflect.New(column.ScanType()).Interface()
			values[i] = object[column.Name()]

		}

		err = rows.Scan(values...)
		if err != nil {
			return []pkg.PostgresResults{}, err
		}

		objects = append(objects, object)

		var result pkg.PostgresResults
		result.Values = make(map[string]string)
		for key, value := range object {
			//var v *interface{} = (*interface{})(reflect.ValueOf(value))
			switch v := value.(type) {
			case *interface{}:
				result.Values[key] = fmt.Sprintf("%v", *v)
			case *time.Time:
				result.Values[key] = fmt.Sprintf("%v", v)
			default:
				log.Fatalf("I don't know about type %T!\n", v)
			}

		}

		results = append(results, result)

	}

	yaml, err := yaml.Marshal(objects)
	log.Info(string(yaml))
	log.Debugf("Connection test query result of %v", objects)
	return results, nil
}

// Obfuscate passwords of the form ' password=xxxxx ' from connectionString since
// connectionStrings are used as metric labels and we don't want to leak passwords
// Return: the connectionString with the password replaced by '###'
func obfuscateConnectionStringPassword(connectionString string) string {
	//looking for a substring that starts with a space,
	//'password=', then any non-whitespace characters,
	//until an ending space
	re := regexp.MustCompile(`\spassword=\S*\s`)
	return re.ReplaceAllString(connectionString, " password=### ")
}
