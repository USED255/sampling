package main

import (
	"bytes"
	"log"
	"net/http"

	"github.com/d2r2/go-bh1750"
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
	"github.com/robfig/cron"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sensor_sampling_data struct {
	gorm.Model
	Temperature float32 `gorm:"default:null"`
	Humidity    float32 `gorm:"default:null"`
	Pressure    float32 `gorm:"default:null"`
	Altitude    float32 `gorm:"default:null"`
	Illuminance float32 `gorm:"default:null"`
}

type caiyun_sampling_date struct {
	gorm.Model
	Response string `gorm:"default:null"`
}

var db *gorm.DB
var err error

func main() {
	db, err = gorm.Open(sqlite.Open("sensor.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&sensor_sampling_data{})

	sensor_sampling_task := cron.New()
	err = sensor_sampling_task.AddFunc("0 0/1 * * * ?", sensor_sampling)
	if err != nil {
		log.Fatalln(err)
	}
	sensor_sampling_task.Start()

	caiyun_sampling_task := cron.New()
	err = caiyun_sampling_task.AddFunc("0 0/15 * * * ?", caiyun_sampling)
	if err != nil {
		log.Fatalln(err)
	}
	caiyun_sampling_task.Start()

	sensor_sampling()
	caiyun_sampling()
	defer sensor_sampling_task.Stop()
	defer caiyun_sampling_task.Stop()
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

func sensor_sampling() {
	temperature, altitude, pressure, err := bmp280_sampling()
	if err != nil {
		log.Println(err)
		return
	}
	illuminance, err := bh1750_sampling()
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Temperature = %v*C, Humidity = %v%%, Pressure = %vPa, Altitude=%vm, Illuminance= %vlux \n",
		temperature, nil, pressure, altitude, float32(illuminance))
	db.Create(&sensor_sampling_data{Temperature: temperature, Pressure: pressure, Altitude: altitude, Illuminance: float32(illuminance)})
}

func caiyun_sampling() {
	url := "https://api.caiyunapp.com/v2.5/OsND6NQQTmhh2yde/117.559364,39.764/realtime.json"
	response, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	if err != nil {
		log.Println(err)
		return
	}
	res := buf.String()
	log.Println(res)
	db.Create(&caiyun_sampling_date{Response: string(res)})
}
