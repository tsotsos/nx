package nx

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"reflect"
	"testing"
)

func init() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}
}

// TestSystemStatus tests Nx System Status retrieval
func TestSystemStatus(t *testing.T) {
	settings := Settings{
		Protocol: getEnv("NX_PROTOCOL", ""),
		Host:     getEnv("NX_HOST", ""),
		Name:     getEnv("NX_NANE", ""),
		User:     getEnv("NX_USER", ""),
		Pin:      getEnv("NX_PIN", ""),
		URL: getEnv("NX_PROTOCOL", "") + "://" +
			getEnv("NX_HOST", "") + "/",
	}
	nx := NewAlarm(settings)
	data, err := nx.SystemStatus()
	if err != nil {
		fmt.Println(err)
		return
	}
	v := reflect.ValueOf(data.System)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	typeOfS := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fmt.Printf("Field: %10s\tValue: %10v\n", typeOfS.Field(i).Name,
			v.Field(i).Interface())
	}
}

func TestZoneSatus(t *testing.T) {
	settings := Settings{
		Protocol: getEnv("NX_PROTOCOL", ""),
		Host:     getEnv("NX_HOST", ""),
		Name:     getEnv("NX_NANE", ""),
		User:     getEnv("NX_USER", ""),
		Pin:      getEnv("NX_PIN", ""),
		URL: getEnv("NX_PROTOCOL", "") + "://" +
			getEnv("NX_HOST", "") + "/",
	}
	nx := NewAlarm(settings)
	data, err := nx.ZonesStatus()
	if err != nil {
		fmt.Println(err)
		return
	}
	//zone names
	for i, v := range data.Zones.Names {
		zone := i + 1
		status := ""
		if i < len(data.Zones.Status) {
			status = fmt.Sprintf("%+v", data.Zones.Status[i])
		}
		fmt.Println("zone: ", zone, v, "\t\t", status)

	}
}

// returns Environment (string) variable or default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}
