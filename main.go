package main

import (
	"bytes"
	"log"
	"net/http"

	"github.com/d2r2/go-bh1750"
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sensor_sampling_data struct {
	gorm.Model
	Temperature       float32 //`gorm:"default:null"`
	AHT20_Temperature float32 //`gorm:"default:null"`
	Humidity          float32 //`gorm:"default:null"`
	Pressure          float32 //`gorm:"default:null"`
	Altitude          float32 //`gorm:"default:null"`
	Illuminance       float32 //`gorm:"default:null"`
}

type caiyun_sampling_date struct {
	gorm.Model
	Response string `gorm:"default:null"`
}

var db *gorm.DB
var err error

func main() {
	defer log.Println("再见👋")
	db, err = gorm.Open(sqlite.Open("sensor.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&sensor_sampling_data{}, &caiyun_sampling_date{})

	sensor_sampling_task := cron.New()
	err = sensor_sampling_task.AddFunc("0 0/1 * * * ?", func() { sensor_sampling_with_retry(3) })
	if err != nil {
		log.Fatalln(err)
	}
	sensor_sampling()
	sensor_sampling_task.Start()
	defer sensor_sampling_task.Stop()

	caiyun_sampling_task := cron.New()
	err = caiyun_sampling_task.AddFunc("0 0/15 * * * ?", func() { caiyun_sampling_with_retry(3) })
	if err != nil {
		log.Fatalln(err)
	}
	caiyun_sampling()
	caiyun_sampling_task.Start()
	defer caiyun_sampling_task.Stop()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/ping", ping)
	api := r.Group("/api/v1")
	{
		api.GET("/environmental_sampling_data", Get_environmental_sampling_data)
	}
	r.Run()

	select {}
}

func bh1750_sampling() (uint16, error) {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bh1750", logger.InfoLevel)
	i2c, err := i2c.NewI2C(0x23, 1)
	if err != nil {
		log.Fatal(err)
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

func aht20_sampling() (float32, float32, error) {
	bus, err := i2c.NewI2C(0x38, 1)
	if err != nil {
		return 0, 0, err
	}
	aht20 := AHT20New(bus)
	err = aht20.ReadWithRetry(3)
	if err != nil {
		return 0, 0, err
	}
	return aht20.Celsius(), aht20.RelHumidity(), nil
}

func sensor_sampling() error {
	temperature, altitude, pressure, err := bmp280_sampling()
	if err != nil {
		return err
	}
	illuminance, err := bh1750_sampling()
	if err != nil {
		return err
	}
	aht20_temperature, humidity, err := aht20_sampling()
	if err != nil {
		return err
	}
	log.Printf("Temperature = %v*C, AHT20-Temperature = %v*C, Humidity = %v%%, Pressure = %vPa, Altitude=%.2fm, Illuminance= %vlux \n",
		temperature, aht20_temperature, humidity, pressure, altitude/10, float32(illuminance))
	db.Create(&sensor_sampling_data{Temperature: temperature, AHT20_Temperature: aht20_temperature, Humidity: humidity, Pressure: pressure, Altitude: altitude, Illuminance: float32(illuminance)})
	return nil
}

func sensor_sampling_with_retry(i int) {
	err := sensor_sampling()
	for {
		if err == nil {
			return
		}
		if i == 0 {
			log.Panicln(err)
			return
		}
		err = sensor_sampling()
		i = i - 1
	}
}

func caiyun_sampling() error {
	url := "https://api.caiyunapp.com/v2.5/OsND6NQQTmhh2yde/117.559364,39.764/realtime.json"
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	if err != nil {
		return err
	}
	res := buf.String()
	log.Println(res)
	db.Create(&caiyun_sampling_date{Response: string(res)})
	return nil
}

func caiyun_sampling_with_retry(i int) {
	err := caiyun_sampling()
	for {
		if err == nil {
			return
		}
		if i == 0 {
			log.Panicln(err)
			return
		}
		err = caiyun_sampling()
		i = i - 1
	}
}

func ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

func Get_environmental_sampling_data(c *gin.Context) {

}
