package main

import (
	"context"
	"encoding/json"
	"errors"
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
				log.Println(trace(1), err)
				continue
			}

			fmt.Println(historicalDataURI)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Println(trace(1), err)
				continue
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Println(trace(1), "status code error: %d %s", resp.StatusCode, resp.Status)
				continue
			}

			var h HourlyWeatherData
			if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
				log.Println(trace(1), err)
				continue
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
			log.Println(trace(2), err)
			return
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
		return UniversalWeatherData{}, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return UniversalWeatherData{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("status code error: %d %s", resp.StatusCode, resp.Status)
		return UniversalWeatherData{}, errors.New("invalid_data")

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
