package handlers

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Calgorr/EnglishPinglish/config"
	"github.com/Calgorr/EnglishPinglish/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	cfg           *config.Config
	router        *gin.Engine
	wordsRepo     repositories.WordsRepository
	client        *http.Client
	totalRequests *prometheus.CounterVec
	redisHits     *prometheus.CounterVec
	errors        *prometheus.CounterVec
	latency       *prometheus.HistogramVec
}

func NewServer(cfg *config.Config) *Server {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

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
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_latency",
			Help:    "API latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	prometheus.MustRegister(totalRequests, redisHits, errors, latency)

	router := gin.Default()

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return &Server{
		cfg:           cfg,
		router:        router,
		wordsRepo:     repositories.NewWordsRepository(cfg.Redis),
		client:        client,
		totalRequests: totalRequests,
		redisHits:     redisHits,
		errors:        errors,
		latency:       latency,
	}
}

func (s *Server) Start() error {
	s.router.GET("/dictionary/:word", s.GetWordFromDictionary)
	s.router.GET("/random", s.GetRandomWord)

	return s.router.Run(":" + s.cfg.Server.Port)
}

func (s *Server) GetWordFromDictionary(c *gin.Context) {
	start := time.Now()
	defer func() { s.latency.WithLabelValues("GetRandomWord").Observe(time.Since(start).Seconds()) }()
	s.totalRequests.WithLabelValues("GetWordFromDictionary").Inc()
	word := c.Param("word")
	if word == "" {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusBadRequest, "Word is empty")
		return
	}

	result, err := s.wordsRepo.GetWord(c.Request.Context(), word)
	if err == nil {
		s.redisHits.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusOK, "from redis: "+result)
		return
	}

	req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+word, nil)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err := s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, "Failed to get word from dictionary")
		return
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	dictResponse := struct {
		Definition string `json:"definition"`
	}{}
	if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
	if err = s.wordsRepo.SetWord(c.Request.Context(), word, dictResponse.Definition, ttl); err != nil {
		s.errors.WithLabelValues("GetWordFromDictionary").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.String(http.StatusOK, "from ninja: "+dictResponse.Definition)
}

func (s *Server) GetRandomWord(c *gin.Context) {
	start := time.Now()
	defer func() { s.latency.WithLabelValues("GetRandomWord").Observe(time.Since(start).Seconds()) }()
	s.totalRequests.WithLabelValues("GetRandomWord").Inc()

	req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaRandomURL, nil)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err := s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	randomResponse := struct {
		Word []string `json:"word"`
	}{}
	if err = json.Unmarshal(jsonData, &randomResponse); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	result, err := s.wordsRepo.GetWord(c.Request.Context(), randomResponse.Word[0])
	if err == nil {
		s.redisHits.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusOK, "word: "+randomResponse.Word[0]+" from redis: "+result)
		return
	}

	req, err = http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+randomResponse.Word[0], nil)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err = s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	jsonData, err = io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	dictResponse := struct {
		Definition string `json:"definition"`
	}{}
	if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
	if err = s.wordsRepo.SetWord(c.Request.Context(), randomResponse.Word[0], dictResponse.Definition, ttl); err != nil {
		s.errors.WithLabelValues("GetRandomWord").Inc()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.String(http.StatusOK, "word: "+randomResponse.Word[0]+" from ninja: "+dictResponse.Definition)
}
