package main

import (
	"log"

	"github.com/d2r2/go-bh1750"
	"github.com/d2r2/go-bsbmp"
	"github.com/d2r2/go-i2c"
	"github.com/robfig/cron"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sensor_sampling_data struct {
	gorm.Model
	Temperature float32
	Humidity    float32
	Pressure    float32
	Illuminance float32
}

var db *gorm.DB
var err error

func main() {
	db, err = gorm.Open(sqlite.Open("sensor.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&sensor_sampling_data{})

	sensor_task := cron.New()
	err = sensor_task.AddFunc("0 0/10 * * * ?", sampling)
	if err != nil {
		log.Fatalln(err)
	}
	sensor_task.Start()
	sampling()
	defer sensor_task.Stop()
	select {}
}

func bh() uint16 {
	i2c, err := i2c.NewI2C(0x23, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()

	sensor := bh1750.NewBH1750()

	resolution := bh1750.HighResolution
	amb, err := sensor.MeasureAmbientLight(i2c, resolution)
	if err != nil {
		log.Fatal(err)
	}

	return amb
}

func bmp() (float32, float32, float32) {
	i2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer i2c.Close()

	sensor, err := bsbmp.NewBMP(bsbmp.BMP280, i2c)
	if err != nil {
		log.Fatal(err)
	}

	temperature, err := sensor.ReadTemperatureC(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		log.Fatal(err)
	}
	pressure, err := sensor.ReadPressurePa(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		log.Fatal(err)
	}
	altitude, err := sensor.ReadAltitude(bsbmp.ACCURACY_STANDARD)
	if err != nil {
		log.Fatal(err)
	}

	return temperature, altitude, pressure
}

func sampling() {
	temperature, _, pressure := bmp()
	illuminance := bh()

	log.Printf("Temperature = %v*C, Humidity = %v%%, Pressure = %vPa, Illuminance= %vlux \n",
		temperature, nil, pressure, float32(illuminance))
	db.Create(&sensor_sampling_data{Temperature: temperature, Pressure: pressure, Illuminance: float32(illuminance)})
}
