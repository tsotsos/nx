package nx

import (
	"github.com/joho/godotenv"
	"testing"
)

func init() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}
}

func TestNx(t *testing.T) {
	nx := NewNxAlarm()
	nx.SetSystem(Chime)
}
