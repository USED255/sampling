package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

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
	Time_Consuming    float32 //`gorm:"default:null"`
	bmp280_err        string  `gorm:"default:null"`
	bh1750_err        string  `gorm:"default:null"`
	aht20_err         string  `gorm:"default:null"`
}

type caiyun_sampling_date struct {
	gorm.Model
	Response       string  `gorm:"default:null"`
	Time_Consuming float32 `gorm:"default:null"`
	Err            string  `gorm:"default:null"`
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
	err = sensor_sampling_task.AddFunc("0 0/1 * * * ?", sensor_sampling_work)
	if err != nil {
		log.Fatalln(err)
	}
	sensor_sampling_work()
	sensor_sampling_task.Start()
	defer sensor_sampling_task.Stop()

	caiyun_sampling_task := cron.New()
	err = caiyun_sampling_task.AddFunc("0 0/15 * * * ?", caiyun_sampling_work)
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
		api.GET("/current_environmental_sampling_data", Get_current_environmental_sampling_data)
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
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
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

func sensor_sampling() *sensor_sampling_data {
	t1 := time.Now()
	temperature, altitude, pressure, bmp280_err := bmp280_sampling()
	if err != nil {
		log.Println(bmp280_err)
	}
	illuminance, bh1750_err := bh1750_sampling()
	if err != nil {
		log.Println(bh1750_err)
	}
	aht20_temperature, humidity, aht20_err := aht20_sampling()
	if err != nil {
		log.Println(aht20_err)
	}
	elapsed := float32(time.Since(t1).Microseconds()) / float32(1000000)
	return &sensor_sampling_data{Temperature: temperature, AHT20_Temperature: aht20_temperature, Humidity: humidity, Pressure: pressure, Altitude: altitude / 10, Illuminance: float32(illuminance), Time_Consuming: elapsed, bmp280_err: bmp280_err.Error(), bh1750_err: bh1750_err.Error(), aht20_err: aht20_err.Error()}
}

func sensor_sampling_work() {
	data := sensor_sampling()
	log.Printf("Temperature = %v*C, AHT20-Temperature = %v*C, Humidity = %v%%, Pressure = %vPa, Altitude = %.2fm, Illuminance = %vlux, Time-consuming = %vs\n",
		data.Temperature, data.AHT20_Temperature, data.Humidity, data.Pressure, data.Altitude/10, float32(data.Illuminance), data.Time_Consuming)
	db.Create(data)
}

func caiyun_sampling() *caiyun_sampling_date {
	t1 := time.Now()
	url := "https://api.caiyunapp.com/v2.5/OsND6NQQTmhh2yde/117.559364,39.764/realtime.json"
	response, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return &caiyun_sampling_date{Time_Consuming: float32(time.Since(t1).Microseconds()) * float32(1000000), Err: err.Error()}
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	if err != nil {
		log.Println(err)
		return &caiyun_sampling_date{Time_Consuming: float32(time.Since(t1).Microseconds()) * float32(1000000), Err: err.Error()}
	}
	res := buf.String()
	elapsed := float32(time.Since(t1).Microseconds()) / float32(1000000)
	return &caiyun_sampling_date{Response: res, Time_Consuming: elapsed}
}

func caiyun_sampling_work() {
	db.Create(caiyun_sampling())
}

func ping(c *gin.Context) {
	//c.String(http.StatusOK, "pong")
	res := gin.H{"status": http.StatusOK, "time": time.Now().Format(time.RFC3339), "message": "pong"}
	log.Println(res)
	c.JSON(http.StatusOK, res)
}

func Get_environmental_sampling_data(c *gin.Context) {

}

func Get_current_environmental_sampling_data(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "time": time.Now().Format(time.RFC3339), "sensor_sampling": sensor_sampling(), "caiyun_sampling": caiyun_sampling()})
}
