package Nx

import (
	"fmt"
	"github.com/joho/godotenv"
	"testing"
)

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println(err)
	}
}

func TestNx(t *testing.T) {
	nx := NewNxAlarm()
	nx.SetSystem(Chime)
}
