package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
