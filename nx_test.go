package nx

import (
	"fmt"
	"github.com/joho/godotenv"
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
	nx := NewAlarm()
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
	nx := NewAlarm()
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
