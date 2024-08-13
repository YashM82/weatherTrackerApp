package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type apiConfigData struct {
	OpenWeatherApiKey string `json:"OpenWeatherApiKey"`
}

type weatherData struct {
	Address           string `json:"address"`
	CurrentConditions struct {
		Temperature float64 `json:"temp"`
		Humidity    float64 `json:"humidity"`
		WindSpeed   float64 `json:"wspd"`
		Description string  `json:"conditions"`
	} `json:"currentConditions"`
}


func loadApiConfig(filename string) (apiConfigData, error) {

	bytes, err := os.ReadFile(filename)
	if err != nil {
		return apiConfigData{}, err
	}
	var c apiConfigData
	err = json.Unmarshal(bytes, &c)
	if err != nil {
		return apiConfigData{}, err
	}
	return c, nil

}
func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello from yash go! \n"))
}

// Function to get data from Redis
func getWeatherFromCache(client *redis.Client, city string) (string, error) {
	ctx := context.Background()
	return client.Get(ctx, city).Result()
}

// Function to set data to Redis
func setWeatherToCache(client *redis.Client, city string, data string) error {
	ctx := context.Background()
	return client.Set(ctx, city, data, 10*time.Minute).Err()
}



func query(client *redis.Client, city string) (weatherData, error) {
	// First check if the data is in the cache
	cachedData, err := getWeatherFromCache(client, city)
	if err == nil {
		var d weatherData
		if err := json.Unmarshal([]byte(cachedData), &d); err == nil {
			return d, nil
		}
	}

	// If not in cache, load the API config
	apiConfig, err := loadApiConfig(".apiConfig")
	if err != nil {
		return weatherData{}, err
	}

	// Make the API request
	url := fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s?unitGroup=metric&key=%s&contentType=json", city, apiConfig.OpenWeatherApiKey)
	resp, err := http.Get(url)
	if err != nil {
		return weatherData{}, err
	}
	defer resp.Body.Close()

	var d weatherData
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return weatherData{}, err
	}

	// Cache the response
	jsonData, err := json.Marshal(d)
	if err == nil {
		setWeatherToCache(client, city, string(jsonData))
	}

	return d, nil
}

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	// Ping Redis to ensure it's connected
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("Could not connect to Redis:", err)
		return
	}

	http.HandleFunc("/hello", hello)
	http.HandleFunc("/weather/",
		func(w http.ResponseWriter, r *http.Request) {
			city := strings.SplitN(r.URL.Path, "/weather/", 2)[1]
			data, err := query(client, city) // Pass Redis client
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(data)
		})

	fmt.Println("Server is running on port 8080")
	http.ListenAndServe(":8080", nil)
}
