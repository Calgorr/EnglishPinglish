package handlers

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Calgorr/EnglishPinglish/config"
	"github.com/Calgorr/EnglishPinglish/internal/repositories"
	pm "github.com/labstack/echo-contrib/prometheus" // Prometheus middleware
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics
)

// Server struct definition remains the same
type Server struct {
	cfg           *config.Config
	echo          *echo.Echo
	wordsRepo     repositories.WordsRepository
	client        *http.Client
	totalRequests *prometheus.CounterVec
	redisHits     *prometheus.CounterVec
	errors        *prometheus.CounterVec
}

func NewServer(cfg *config.Config) *Server {
	// Create an HTTP client that skips TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Create Prometheus metrics
	totalRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_total_requests",
			Help: "Total number of requests for each API endpoint",
		},
		[]string{"endpoint"},
	)
	redisHits := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_redis_hits",
			Help: "Number of requests answered by Redis",
		},
		[]string{"endpoint"},
	)
	errors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_errors",
			Help: "Number of unsuccessful responses",
		},
		[]string{"endpoint"},
	)

	// Register metrics with Prometheus
	prometheus.MustRegister(totalRequests, redisHits, errors)

	// Set up Echo with Prometheus middleware
	e := echo.New()
	p := pm.NewPrometheus("echo", nil)
	p.Use(e)

	return &Server{
		cfg:           cfg,
		echo:          e,
		wordsRepo:     repositories.NewWordsRepository(cfg.Redis),
		client:        client,
		totalRequests: totalRequests,
		redisHits:     redisHits,
		errors:        errors,
	}
}

func (s *Server) Start() error {
	s.echo.Use(middleware.Logger())
	s.echo.GET("/dictionary/:word", s.GetWordFromDictionary)
	s.echo.GET("/random", s.GetRandomWord)

	return s.echo.Start(":" + s.cfg.Server.Port)
}

func (s *Server) GetWordFromDictionary(c echo.Context) error {
	s.totalRequests.WithLabelValues("GetWordFromDictionary").Inc() // Increment total requests
	word := c.Param("word")
	if word == "" {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc() // Log error for empty word
		return c.String(400, "Word is empty")
	}

	result, err := s.wordsRepo.GetWord(c.Request().Context(), word)
	if err == nil {
		s.redisHits.WithLabelValues("GetWordFromDictionary").Inc() // Log Redis hit
		return c.String(200, "from redis: "+result)
	}

	req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+word, nil)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, err.Error())
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err := s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, "Failed to get word from dictionary")
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, err.Error())
	}

	dictResponse := struct {
		Definition string `json:"definition"`
	}{}
	if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, err.Error())
	}

	ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
	if err = s.wordsRepo.SetWord(c.Request().Context(), word, dictResponse.Definition, ttl); err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		return c.String(500, err.Error())
	}

	return c.String(200, "from ninja: "+dictResponse.Definition)
}

func (s *Server) GetRandomWord(c echo.Context) error {
	s.totalRequests.WithLabelValues("GetRandomWord").Inc() // Increment total requests

	req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaRandomURL, nil)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err := s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	randomResponse := struct {
		Word []string `json:"word"`
	}{}
	if err = json.Unmarshal(jsonData, &randomResponse); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	result, err := s.wordsRepo.GetWord(c.Request().Context(), randomResponse.Word[0])
	if err == nil {
		s.redisHits.WithLabelValues("GetRandomWord").Inc()
		return c.String(200, "word: "+randomResponse.Word[0]+" from redis: "+result)
	}

	req, err = http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+randomResponse.Word[0], nil)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err = s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}
	defer resp.Body.Close()

	jsonData, err = io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	dictResponse := struct {
		Definition string `json:"definition"`
	}{}
	if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
	if err = s.wordsRepo.SetWord(c.Request().Context(), randomResponse.Word[0], dictResponse.Definition, ttl); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		return c.String(500, err.Error())
	}

	return c.String(200, "word: "+randomResponse.Word[0]+" from ninja: "+dictResponse.Definition)
}
