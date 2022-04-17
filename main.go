package main

import (
	"flag"
	"log"

	"github.com/d2r2/go-bh1750"
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
	"github.com/gin-gonic/gin"
	aht20 "github.com/used255/aht20-go"
)

func main() {
	bindFlagPtr := flag.String("bind", ":8080", "bind address")
	disableGinModeFlagPtr := flag.Bool("disable-gin-debug-mode", false, "gin.ReleaseMode")
	flag.Parse()
	if *disableGinModeFlagPtr {
		gin.SetMode(gin.ReleaseMode)
	}
	log.Println("Welcome 🐱‍🏍")
	r := gin.Default()
	//r.SetTrustedProxies([]string{"192.168.0.0/24", "172.16.0.0/12", "10.0.0.0/8"})
	api := r.Group("/api/v1")
	{
		api.GET("/ping",
			func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "pong",
				})
			})
		api.GET("/sampling", sampling)
	}
	r.Run(*bindFlagPtr)
}

func sampling(c *gin.Context) {
	c.JSON(200, sampling_json())
}

func sampling_json() gin.H {
	return gin.H{
		"aht20":  aht20_sampling_json(),
		"bh1750": bh1750_sampling_json(),
		"bmp280": bmp280_sampling_json(),
	}
}

func aht20_sampling_json() gin.H {
	temperature, humidity, err := aht20_sampling()
	j := gin.H{"temperature": temperature, "humidity": humidity}
	if err != nil {
		j["error"] = err.Error()
	}
	return j
}

func bh1750_sampling_json() gin.H {
	amb, err := bh1750_sampling()
	j := gin.H{"ambient": amb}
	if err != nil {
		j["error"] = err.Error()
	}
	return j
}

func bmp280_sampling_json() gin.H {
	temperature, altitude, pressure, err := bmp280_sampling()
	j := gin.H{"temperature": temperature, "altitude": altitude, "pressure": pressure}
	if err != nil {
		j["error"] = err.Error()
	}
	return j
}

func aht20_sampling() (float32, float32, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("aht20", logger.InfoLevel)
	bus, err := i2c.NewI2C(0x38, 1)
	if err != nil {
		return 0, 0, err
	}
	aht20 := aht20.AHT20New(bus)
	err = aht20.ReadWithRetry(3)
	if err != nil {
		return 0, 0, err
	}
	return aht20.Celsius(), aht20.RelHumidity(), nil
}

func bh1750_sampling() (uint16, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bh1750", logger.InfoLevel)

	i2c, err := i2c.NewI2C(0x23, 1)
	if err != nil {
		return 0, err
	}
	defer i2c.Close()

	sensor := bh1750.NewBH1750()

	resolution := bh1750.HighResolution
	amb, err := sensor.MeasureAmbientLight(i2c, resolution)
	if err != nil {
		return 0, err
	}

	return amb, nil
}

func bmp280_sampling() (float32, float32, float32, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bsbmp", logger.InfoLevel)

	i2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		return 0, 0, 0, err
	}
	defer i2c.Close()

	sensor, err := bsbmp.NewBMP(bsbmp.BMP280, i2c)
	if err != nil {
		return 0, 0, 0, err
	}

	temperature, err := sensor.ReadTemperatureC(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}
	pressure, err := sensor.ReadPressurePa(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}
	altitude, err := sensor.ReadAltitude(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		return 0, 0, 0, err
	}

	return temperature, altitude, pressure, nil
}
