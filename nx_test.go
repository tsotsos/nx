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

func TestGetSystemStatus(t *testing.T) {
	nx := NewNxAlarm()
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
