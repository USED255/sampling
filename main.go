package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
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
var logg *log.Logger

const (
	//I = "\033[0m\n\033[1;32m[I]"
	I string = ""
	W string = "\033[0m\n\032[1;32m[W]"
	F string = "\033[0m\n\031[1;32m[F]"
)

func main() {
	logg = log.New(io.MultiWriter(os.Stdout), I, log.Ldate|log.Ltime|log.Lshortfile)
	logg.SetPrefix(I)
	logg.Println("欢迎😺")
	defer logg.Println("再见👋")
	db, err = gorm.Open(sqlite.Open("sensor.db"), &gorm.Config{})
	if err != nil {
		logg.SetPrefix(F)
		logg.Fatal("failed to connect database")
	}
	db.AutoMigrate(&sensor_sampling_data{}, &caiyun_sampling_date{})

	sensor_sampling_task := cron.New()
	err = sensor_sampling_task.AddFunc("0 0/1 * * * ?", sensor_sampling_work)
	if err != nil {
		logg.SetPrefix(F)
		logg.Fatalln(err)
	}
	sensor_sampling_work()
	sensor_sampling_task.Start()
	defer sensor_sampling_task.Stop()

	caiyun_sampling_task := cron.New()
	err = caiyun_sampling_task.AddFunc("0 0/15 * * * ?", caiyun_sampling_work)
	if err != nil {
		logg.SetPrefix(F)
		logg.Fatalln(err)
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
	t2 := func(err error) string {
		if err != nil {
			return err.Error()
		}
		return ""
	}
	t1 := time.Now()
	temperature, altitude, pressure, bmp280_err := bmp280_sampling()
	if bmp280_err != nil {
		logg.SetPrefix(W)
		logg.Println(bmp280_err)
	}
	illuminance, bh1750_err := bh1750_sampling()
	if bh1750_err != nil {
		logg.SetPrefix(W)
		logg.Println(bh1750_err)
	}
	aht20_temperature, humidity, aht20_err := aht20_sampling()
	if aht20_err != nil {
		logg.SetPrefix(W)
		logg.Println(aht20_err)
	}
	elapsed := float32(time.Since(t1).Microseconds()) / float32(1000000)
	return &sensor_sampling_data{Temperature: temperature, AHT20_Temperature: aht20_temperature, Humidity: humidity, Pressure: pressure, Altitude: altitude / 10, Illuminance: float32(illuminance), Time_Consuming: elapsed, bmp280_err: t2(bmp280_err), bh1750_err: t2(bh1750_err), aht20_err: t2(aht20_err)}
}

func sensor_sampling_work() {
	data := sensor_sampling()
	logg.SetPrefix(string(I))
	logg.Printf("Temperature = %v*C, AHT20-Temperature = %.2f*C, Humidity = %.2f%%, Pressure = %vPa, Altitude = %.2fm, Illuminance = %vlux, Time-consuming = %vs\n",
		data.Temperature, data.AHT20_Temperature, data.Humidity, data.Pressure, data.Altitude/10, float32(data.Illuminance), data.Time_Consuming)
	db.Create(data)
}

func caiyun_sampling() *caiyun_sampling_date {
	t1 := time.Now()
	url := "https://api.caiyunapp.com/v2.5/OsND6NQQTmhh2yde/117.559364,39.764/realtime.json"
	response, err := http.Get(url)
	if err != nil {
		logg.SetPrefix(W)
		logg.Println(err)
		return &caiyun_sampling_date{Time_Consuming: float32(time.Since(t1).Microseconds()) * float32(1000000), Err: err.Error()}
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(response.Body)
	if err != nil {
		logg.SetPrefix(W)
		logg.Println(err)
		return &caiyun_sampling_date{Time_Consuming: float32(time.Since(t1).Microseconds()) * float32(1000000), Err: err.Error()}
	}
	res := buf.String()
	elapsed := float32(time.Since(t1).Microseconds()) / float32(1000000)
	return &caiyun_sampling_date{Response: res, Time_Consuming: elapsed}
}

func caiyun_sampling_work() {
	res := caiyun_sampling()
	logg.SetPrefix(I)
	logg.Println(res)
	db.Create(res)
}

func ping(c *gin.Context) {
	//c.String(http.StatusOK, "pong")
	res := gin.H{"status": http.StatusOK, "time": time.Now().Format(time.RFC3339), "message": "pong"}
	logg.SetPrefix(I)
	logg.Println(res)
	c.JSON(http.StatusOK, res)
}

func Get_environmental_sampling_data(c *gin.Context) {

}

func Get_current_environmental_sampling_data(c *gin.Context) {
	res := gin.H{"status": http.StatusOK, "time": time.Now().Format(time.RFC3339), "sensor_sampling": sensor_sampling(), "caiyun_sampling": caiyun_sampling()}
	logg.SetPrefix(I)
	logg.Println(res)
	c.JSON(http.StatusOK, res)
}
