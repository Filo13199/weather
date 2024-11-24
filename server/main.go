package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OpenWeatherMapData struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Base string `json:"base"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
		SeaLevel  int     `json:"sea_level"`
		GrndLevel int     `json:"grnd_level"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
		Gust  float64 `json:"gust"`
	} `json:"wind"`
	Rain struct {
		OneH float64 `json:"1h"`
	} `json:"rain"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Dt  int64 `json:"dt"`
	Sys struct {
		Type    int    `json:"type"`
		ID      int    `json:"id"`
		Country string `json:"country"`
		Sunrise int64  `json:"sunrise"`
		Sunset  int64  `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Cod      int    `json:"cod"`
}

// // "lat":51.5073219,
//       "lon":-0.1276474,
//       "country":"GB",
//       "state":"England"

type CityGeoData struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Country string  `json:"country"`
	State   string  `json:"state"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type UniversalWeatherData struct {
	FeelsLike                float64             `json:"feels_like"`
	TempMin                  float64             `json:"temp_min"`
	TempMax                  float64             `json:"temp_max"`
	Pressure                 int                 `json:"pressure"`
	Humidity                 int                 `json:"humidity"`
	Temp                     float64             `json:"temp"`
	CityName                 string              `json:"city_name"`
	History                  []HourlyWeatherData `json:"history"`
	SourceResponse           interface{}         `json:"source_response"`
	Source                   string              `json:"source"`
	PredictedWeatherNextHour float64             `json:"pred_weather_next_hr"`
}

type service struct {
	mongo *mongo.Client
}

type HourlyWeatherData struct {
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	GenerationTimeMs     float64 `json:"generationtime_ms"`
	UtcOffsetSeconds     int     `json:"utc_offset_seconds"`
	Timezone             string  `json:"timezone"`
	TimezoneAbbreviation string  `json:"timezone_abbreviation"`
	Elevation            float64 `json:"elevation"`
	HourlyUnits          struct {
		Time          string `json:"time"`
		Temperature2m string `json:"temperature_2m"`
	} `json:"hourly_units"`
	Hourly struct {
		Time                 []string  `json:"time"`
		Temperature2m        []float64 `json:"temperature_2m"`
		StdDevDiffPercentage []float64 `json:"std_dev_diff_percentage"`
	} `json:"hourly"`
	StandardDeviation float64 `json:"standard_deviation"`
}

type City struct {
	ID                             primitive.ObjectID   `bson:"_id" json:"_id"`
	City                           string               `bson:"city" json:"city"`
	CityAscii                      string               `bson:"city_ascii" json:"city_ascii"`
	Lat                            float64              `bson:"lat" json:"lat"`
	Lng                            float64              `bson:"lng" json:"lng"`
	Country                        string               `bson:"country" json:"country"`
	ISO2                           string               `bson:"iso2" json:"iso2"`
	ISO3                           string               `bson:"iso3" json:"iso3"`
	AdminName                      string               `bson:"admin_name" json:"admin_name"`
	Capital                        string               `bson:"capital" json:"capital"`
	Population                     primitive.Decimal128 `bson:"population" json:"population"`
	IDInt                          int                  `bson:"id" json:"id"`
	HourlyAverageStdPercentageDiff []float64            `json:"hourly_avg_std_percentage_diff" bson:"hourly_avg_std_percentage_diff"`
	HistoricalData                 []HourlyWeatherData  `bson:"historical_data" json:"historical_data"`
}

func OpenWeatherMapToUniversalWeatherData(decoder *json.Decoder) (UniversalWeatherData, error) {
	d := OpenWeatherMapData{}
	if err := decoder.Decode(&d); err != nil {
		return UniversalWeatherData{}, err
	}
	return UniversalWeatherData{
		FeelsLike:      d.Main.FeelsLike,
		TempMin:        d.Main.TempMin,
		TempMax:        d.Main.TempMax,
		Pressure:       d.Main.Pressure,
		Humidity:       d.Main.Humidity,
		Temp:           d.Main.Temp,
		CityName:       d.Name,
		SourceResponse: d,
		Source:         "openweathermap",
	}, nil
}

//{"latitude":52.52,"longitude":13.419998,"generationtime_ms":0.033974647521972656,"utc_offset_seconds":0,"timezone":"GMT","timezone_abbreviation":"GMT","elevation":38.0,"current_units":{"time":"iso8601","interval":"seconds","temperature_2m":"°C","wind_speed_10m":"km/h"},"current":{"time":"2024-11-23T18:15","interval":900,"temperature_2m":2.3,"wind_speed_10m":10.7}}

type OpenMeteoData struct {
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	GenerationTimeMs     float64 `json:"generationtime_ms"`
	UtcOffsetSeconds     int     `json:"utc_offset_seconds"`
	Timezone             string  `json:"timezone"`
	TimezoneAbbreviation string  `json:"timezone_abbreviation"`
	Elevation            float64 `json:"elevation"`
	CurrentUnits         struct {
		Time          string `json:"time"`
		Interval      string `json:"interval"`
		Temperature2m string `json:"temperature_2m"`
		WindSpeed10m  string `json:"wind_speed_10m"`
	} `json:"current_units"`
	Current struct {
		Time               string  `json:"time"`
		Interval           int     `json:"interval"`
		Temperature2m      float64 `json:"temperature_2m"`
		WindSpeed10m       float64 `json:"wind_speed_10m"`
		PressureMSL        float64 `json:"pressure_msl"`
		RelativeHumidity2M float64 `json:"relative_humidity_2m"`
	} `json:"current"`
}

func OpenMeteoDataToUniversalWeatherData(decoder *json.Decoder) (UniversalWeatherData, error) {
	d := OpenMeteoData{}
	if err := decoder.Decode(&d); err != nil {
		return UniversalWeatherData{}, err
	}
	return UniversalWeatherData{
		FeelsLike:      d.Current.Temperature2m,
		TempMin:        d.Current.Temperature2m,
		TempMax:        d.Current.Temperature2m,
		Pressure:       int(d.Current.PressureMSL),
		Humidity:       int(d.Current.RelativeHumidity2M),
		Temp:           d.Current.Temperature2m,
		SourceResponse: d,
		Source:         "openmeteo",
	}, nil
}

var (
	openWeatherMapSecretKey = os.Getenv("OPEN_WEATHER_MAP_SECRET_KEY")
	historicalDepth         = 5
)

type Source struct {
	Source string `json:"source"`
	URI    string `json:"uri"`
	// generic conversion function
	DecodeInto func(*json.Decoder) (UniversalWeatherData, error)
}

type AggregatedData struct {
	Sources []UniversalWeatherData `json:"sources"`
}

func (s *service) eventLoop(conn *websocket.Conn, city City) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	currentTime := time.Now()
	sessionId := primitive.NewObjectID()
	if len(city.HistoricalData) == 0 {
		historicalData := make([]HourlyWeatherData, 0, historicalDepth)
		// imaginary lock here to prevent race conditions on same city
		avgStdDev := float64(0)
		for i := 1; i <= historicalDepth; i++ {
			historicalDataURI := fmt.Sprintf("https://archive-api.open-meteo.com/v1/archive?latitude=%f&longitude=%f&start_date=%d-%d-%d&end_date=%d-%d-%d&hourly=temperature_2m", city.Lat, city.Lng, currentTime.Year()-i, currentTime.Month(), currentTime.Day(), currentTime.Year()-i, currentTime.Month(), currentTime.Day())
			req, err := http.NewRequest(http.MethodGet, historicalDataURI, nil)
			if err != nil {
				log.Fatal(trace(1), err)
			}

			fmt.Println(historicalDataURI)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Fatal(trace(1), err)
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Println(trace(1), "status code error: %d %s", resp.StatusCode, resp.Status)
			}

			var h HourlyWeatherData
			if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
				log.Fatal(trace(1), err)
			}

			tempSum := float64(0)
			for _, t := range h.Hourly.Temperature2m {
				tempSum += t
			}

			meanTemp := tempSum / float64(len(h.Hourly.Temperature2m))
			devs := float64(0)
			for _, t := range h.Hourly.Temperature2m {
				variance := t - meanTemp
				devs += math.Pow(variance, 2)
			}

			devs = devs / float64(len(h.Hourly.Temperature2m))
			stdDev := math.Sqrt(devs)
			avgStdDev += stdDev
			for j := 1; j < len(h.Hourly.Temperature2m); j++ {
				diff := h.Hourly.Temperature2m[j] - h.Hourly.Temperature2m[j-1]
				differenceFromStdDevPercentage := (diff / stdDev)
				h.Hourly.StdDevDiffPercentage = append(h.Hourly.StdDevDiffPercentage, differenceFromStdDevPercentage)
			}

			fmt.Printf("standard deviation for %s on %s is = %f \n", city.City, h.Hourly.Time[0], stdDev)
			h.StandardDeviation = stdDev
			historicalData = append(historicalData, h)
		}

		hourlyAvgStdDiffPercentage := []float64{}
		for j := 0; j < 23; j++ {
			sum := float64(0)
			for i := 0; i < len(historicalData); i++ {
				sum += historicalData[i].Hourly.StdDevDiffPercentage[j]
			}
			hourlyAvgStdDiffPercentage = append(hourlyAvgStdDiffPercentage, sum/float64(len(historicalData)))
		}

		col := s.mongo.Database("weather").Collection("cities")
		col.UpdateByID(context.TODO(), city.ID, bson.M{"$set": bson.M{"historical_data": historicalData, "avg_std_dev": avgStdDev / float64(len(historicalData)), "hourly_avg_std_percentage_diff": hourlyAvgStdDiffPercentage}})
		city.HistoricalData = historicalData
		city.HourlyAverageStdPercentageDiff = hourlyAvgStdDiffPercentage
	}
	sources := []Source{
		{
			Source:     "openweathermap",
			URI:        fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&appid=%s&units=metric", city.Lat, city.Lng, openWeatherMapSecretKey),
			DecodeInto: OpenWeatherMapToUniversalWeatherData,
		},
		{
			Source:     "openmeteo",
			URI:        fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,wind_speed_10m", city.Lat, city.Lng),
			DecodeInto: OpenMeteoDataToUniversalWeatherData,
		},
	}

	for {
		<-ticker.C
		universalDataArr := make([]UniversalWeatherData, 0, len(sources))
		for _, source := range sources {
			u, err := getDataFromSource(source, source.DecodeInto)
			if err != nil {
				log.Printf("[%s], [error =%s], [source = %s]", trace(2), err, source.Source)
				continue
			}

			i := (time.Now().Hour() + 1) % 23
			u.PredictedWeatherNextHour = u.Temp * (1 + city.HourlyAverageStdPercentageDiff[i])
			universalDataArr = append(universalDataArr, u)
		}

		res := AggregatedData{
			Sources: universalDataArr,
		}

		col := s.mongo.Database("weather").Collection("weather_sessions")
		// this could be replaced by more mature persistance strategy, maybe producing event on kafka and then doing whatever with it
		_, err := col.UpdateByID(context.TODO(), sessionId, bson.M{"$push": bson.M{"sessionEvents": res}, "$set": bson.M{"cityId": city.ID}}, options.Update().SetUpsert(true))
		if err != nil {
			log.Fatal(trace(2), err)
		}

		if err := conn.WriteJSON(res); err != nil {
			log.Println(err)
			return
		}
	}
}

func getDataFromSource(source Source, convertFn func(*json.Decoder) (UniversalWeatherData, error)) (UniversalWeatherData, error) {
	req, err := http.NewRequest(http.MethodGet, source.URI, nil)
	if err != nil {
		log.Fatal(trace(2), err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(trace(2), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("status code error: %d %s", resp.StatusCode, resp.Status)
	}
	return convertFn(json.NewDecoder(resp.Body))
}

func homePage(c *gin.Context) {
	return
}

func trace(skip int) string {
	skip++
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s- #%d - ?", file, line)
	}

	return fmt.Sprintf("%s- #%d - %s", file, line, fn.Name())
}

func (s *service) wsEndpoint(c *gin.Context) {
	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Print(trace(2), err)
		return
	}

	_, cityId, err := ws.ReadMessage()
	if err != nil {
		log.Print(trace(2), err)
		return
	}

	str := strings.Trim(string(cityId), "\"")

	cityObjId, err := primitive.ObjectIDFromHex(str)
	if err != nil {
		log.Print(trace(2), err)
		return
	}

	col := s.mongo.Database("weather").Collection("cities")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var city City
	err = col.FindOne(ctx, bson.M{"_id": cityObjId}).Decode(&city)
	if err != nil {
		log.Print(trace(2), err)
		return
	}

	log.Println("Client Connected")
	s.eventLoop(ws, city)
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *service) setupRoutes() {
	gin := gin.Default()
	gin.Use(CORSMiddleware())
	gin.GET("/", homePage)
	gin.GET("/ws", s.wsEndpoint)
	gin.GET("/countries", s.getCountries)
	gin.GET("/countries/:country/cities", s.getCities)
	gin.Run(":8000")
}

func (s *service) getCountries(c *gin.Context) {
	collection := s.mongo.Database("weather").Collection("cities")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	countries, err := collection.Distinct(ctx, "country", bson.D{})
	if err != nil {
		log.Println(trace(1), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "tech_error",
		})
		return
	}

	countryList := make([]string, len(countries))
	for i, country := range countries {
		countryList[i] = country.(string)
	}

	c.JSON(http.StatusOK, countryList)
}

func (s *service) getCities(c *gin.Context) {
	country := c.Param("country")
	collection := s.mongo.Database("weather").Collection("cities")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"country": country}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		log.Println(trace(1), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "tech_error",
		})
		return
	}
	defer cursor.Close(ctx)

	var cities []City
	for cursor.Next(ctx) {
		city := City{}
		if err := cursor.Decode(&city); err != nil {
			log.Println(trace(1), err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "tech_error",
			})
			return
		}
		cities = append(cities, city)
	}

	if err := cursor.Err(); err != nil {
		log.Println(trace(1), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "tech_error",
		})
		return
	}

	c.JSON(http.StatusOK, cities)
}

func main() {
	conn, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO_DB_URI")))
	if err != nil {
		log.Fatal(trace(2), err)
	}
	s := service{
		mongo: conn,
	}
	historicalDepthInt, err := strconv.Atoi(os.Getenv("HISTORICAL_DEPTH"))
	if err != nil {
		log.Println(err)
	} else {
		historicalDepth = historicalDepthInt
	}
	s.setupRoutes()
}
