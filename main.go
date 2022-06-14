package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/d2r2/go-bh1750"
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
	"github.com/gin-gonic/gin"
	"github.com/used255/go-aht20"

	"github.com/google/uuid"
)

var lastSamplingData gin.H
var lastSamplingTime int64
var isBusy = false

func main() {
	bindFlagPtr := flag.String("bind", ":8080", "bind address")
	disableGinModeFlagPtr := flag.Bool("disable-gin-debug-mode", false, "gin.ReleaseMode")
	flag.Parse()
	if *disableGinModeFlagPtr {
		gin.SetMode(gin.ReleaseMode)
	}
	log.Println("Welcome 🐱‍🏍")
	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.0/8", "192.168.0.0/24", "172.16.0.0/12", "10.0.0.0/8"})
	api := r.Group("/api/v1")
	{
		api.GET("/ping",
			func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "pong",
				})
			})
		api.GET("/sampling", getSampling)
	}
	r.Run(*bindFlagPtr)
}

func getSampling(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":   http.StatusOK,
		"sampling": sampling(),
	})
}

func sampling() gin.H {
	if isBusy {
		j := lastSamplingData
		j["isBusy"] = true
		return j
	}

	if lastSamplingTime != 0 && getUnixMillisTimestamp()-lastSamplingTime > 1000 {
		j := lastSamplingData
		j["lastSamplingData"] = true
		return j
	}

	isBusy = true
	ts1 := getUnixMillisTimestamp()
	j := gin.H{
		"uuid": uuid.New().String(),
		"ts1":  ts1,
		"sensor": gin.H{
			"aht20":  aht20Sampling(),
			"bh1750": bh1750Sampling(),
			"bmp280": bmp280Sampling(),
		},
	}
	ts2 := getUnixMillisTimestamp()
	j["ts2"] = ts2
	lastSamplingTime = ts2
	lastSamplingData = j
	isBusy = false
	return j
}

func aht20Sampling() gin.H {
	temperature, humidity, err := samplingAHT20()
	if err != nil {
		return gin.H{"error": err.Error()}
	}
	return gin.H{
		"temperature": gin.H{
			"value": temperature,
			"unit":  "°C",
		},
		"humidity": gin.H{
			"value": humidity,
			"unit":  "%",
		},
	}
}

func bh1750Sampling() gin.H {
	amb, err := samplingBH1750()
	if err != nil {
		return gin.H{"error": err.Error()}
	}
	return gin.H{
		"ambient": gin.H{
			"value": amb,
			"unit":  "lx",
		},
	}
}

func bmp280Sampling() gin.H {
	temperature, _, pressure, err := samplingBMP280()
	if err != nil {
		return gin.H{"error": err.Error()}
	}
	return gin.H{
		"temperature": gin.H{
			"value": temperature,
			"unit":  "°C",
		},
		"pressure": gin.H{
			"value": pressure,
			"unit":  "hPa",
		},
	}
}

func samplingAHT20() (float32, float32, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("aht20", logger.InfoLevel)
	bus, err := i2c.NewI2C(0x38, 1)
	if err != nil {
		return 0, 0, err
	}
	s := aht20.NewAHT20(bus)
	err = s.ReadWithRetry(3)
	if err != nil {
		return 0, 0, err
	}
	return s.Celsius(), s.RelHumidity(), nil
}

func samplingBH1750() (uint16, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bh1750", logger.InfoLevel)

	i2c, err := i2c.NewI2C(0x23, 1)
	if err != nil {
		return 0, err
	}
	defer i2c.Close()

	s := bh1750.NewBH1750()
	amb, err := s.MeasureAmbientLight(i2c, bh1750.HighResolution)
	if err != nil {
		return 0, err
	}

	return amb, nil
}

func samplingBMP280() (float32, float32, float32, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bsbmp", logger.InfoLevel)

	i2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		return 0, 0, 0, err
	}
	defer i2c.Close()

	s, err := bsbmp.NewBMP(bsbmp.BMP280, i2c)
	if err != nil {
		return 0, 0, 0, err
	}

	temperature, err := s.ReadTemperatureC(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}
	pressure, err := s.ReadPressurePa(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}
	altitude, err := s.ReadAltitude(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}

	return temperature, altitude, pressure, nil
}

func getUnixMillisTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
