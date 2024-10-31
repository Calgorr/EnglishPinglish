package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Calgorr/EnglishPinglish/config"
	"github.com/Calgorr/EnglishPinglish/internal/repositories"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	cfg       *config.Config
	echo      *echo.Echo
	wordsRepo repositories.WordsRepository
	client    *http.Client // Add custom HTTP client
}

func NewServer(cfg *config.Config) *Server {
	// Create an HTTP client that skips TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return &Server{
		cfg:       cfg,
		echo:      echo.New(),
		wordsRepo: repositories.NewWordsRepository(cfg.Redis),
		client:    client, // Initialize custom client
	}
}

func (s *Server) Start() error {
	s.echo.Use(middleware.Logger())
	s.echo.GET("/dictionary/:word", s.GetWordFromDictionary)
	s.echo.GET("/random", s.GetRandomWord)

	return s.echo.Start(":" + s.cfg.Server.Port)
}

func (s *Server) GetWordFromDictionary(c echo.Context) error {
	word := c.Param("word")
	if word == "" {
		return c.String(400, "Word is empty")
	}

	result, err := s.wordsRepo.GetWord(c.Request().Context(), word)
	if err != nil {
		req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+word, nil)
		if err != nil {
			return c.String(500, err.Error())
		}
		req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
		resp, err := s.client.Do(req) // Use the custom client
		if err != nil {
			return c.String(500, err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			fmt.Println(string(body))
			return c.String(500, "Failed to get word from dictionary")
		}

		jsonData, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.String(500, err.Error())
		}

		dictResponse := struct {
			Definition string `json:"definition"` // Fixed spelling
		}{}

		if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
			return c.String(500, err.Error())
		}
		fmt.Println(dictResponse)

		ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
		if err = s.wordsRepo.SetWord(c.Request().Context(), word, dictResponse.Definition, ttl); err != nil {
			return c.String(500, err.Error())
		}

		return c.String(200, "from ninja: "+dictResponse.Definition)
	}

	return c.String(200, "from redis: "+result)
}

func (s *Server) GetRandomWord(c echo.Context) error {
	req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaRandomURL, nil)
	if err != nil {
		return c.String(500, err.Error())
	}
	req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
	resp, err := s.client.Do(req) // Use the custom client
	if err != nil {
		return c.String(500, err.Error())
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.String(500, err.Error())
	}

	randomResponse := struct {
		Word []string `json:"word"`
	}{}

	if err = json.Unmarshal(jsonData, &randomResponse); err != nil {
		fmt.Println(string(jsonData))
		return c.String(500, err.Error())
	}

	// now pass the word to the dictionary
	result, err := s.wordsRepo.GetWord(c.Request().Context(), randomResponse.Word[0])
	if err != nil {
		req, err := http.NewRequest("GET", s.cfg.Ninja.NinjaDictionaryURL+"/?word="+randomResponse.Word[0], nil)
		if err != nil {
			return c.String(500, err.Error())
		}
		req.Header.Set("X-API-KEY", s.cfg.Ninja.NinjaAPIKey)
		resp, err := s.client.Do(req) // Use the custom client
		if err != nil {
			return c.String(500, err.Error())
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return c.String(500, "Failed to get word from dictionary")
		}

		jsonData, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.String(500, err.Error())
		}

		dictResponse := struct {
			Definition string `json:"definition"` // Fixed spelling
		}{}

		if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
			return c.String(500, err.Error())
		}

		ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
		if err = s.wordsRepo.SetWord(c.Request().Context(), randomResponse.Word[0], dictResponse.Definition, ttl); err != nil {
			return c.String(500, err.Error())
		}

		return c.String(200, "word: "+randomResponse.Word[0]+" from ninja: "+dictResponse.Definition)
	}

	return c.String(200, "word: "+randomResponse.Word[0]+" from redis: "+result)
}
