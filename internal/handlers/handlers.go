package handlers

import (
	"encoding/json"
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
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:       cfg,
		echo:      echo.New(),
		wordsRepo: repositories.NewWordsRepository(cfg.Redis),
	}
}

func (s *Server) Start() error {
	s.echo.Use(middleware.Logger())
	s.echo.GET("/dictionary/:word", s.GetWordFromDictionary)
	s.echo.POST("/random", s.GetRandomWord)

	return s.echo.Start(":" + s.cfg.Server.Port)
}

func (s *Server) GetWordFromDictionary(c echo.Context) error {
	word := c.Param("word")
	if word == "" {
		return c.String(400, "Word is empty")
	}

	result, err := s.wordsRepo.GetWord(c.Request().Context(), word)
	if err != nil {
		resp, err := http.Get(s.cfg.Ninja.NinjaDictionaryURL + "/?word=" + word)
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
			Defenition string `json:"defenition"`
		}{}

		if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
			return c.String(500, err.Error())
		}

		ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
		if err = s.wordsRepo.SetWord(c.Request().Context(), word, dictResponse.Defenition, ttl); err != nil {
			return c.String(500, err.Error())
		}

		return c.String(200, "from ninja: "+dictResponse.Defenition)
	}

	return c.String(200, "from redis: "+result)
}

func (s *Server) GetRandomWord(c echo.Context) error {
	resp, err := http.Get(s.cfg.Ninja.NinjaRandomURL)
	if err != nil {
		return c.String(500, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return c.String(500, "Failed to get random word")
	}

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.String(500, err.Error())
	}

	randomResponse := struct {
		Word string `json:"word"`
	}{}

	if err = json.Unmarshal(jsonData, &randomResponse); err != nil {
		return c.String(500, err.Error())
	}

	// now pass the word to the dictionary
	result, err := s.wordsRepo.GetWord(c.Request().Context(), randomResponse.Word)
	if err != nil {
		resp, err := http.Get(s.cfg.Ninja.NinjaDictionaryURL + "/?word=" + randomResponse.Word)
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
			Defenition string `json:"defenition"`
		}{}

		if err = json.Unmarshal(jsonData, &dictResponse); err != nil {
			return c.String(500, err.Error())
		}

		ttl := time.Duration(s.cfg.Redis.TTL) * time.Second
		if err = s.wordsRepo.SetWord(c.Request().Context(), randomResponse.Word, dictResponse.Defenition, ttl); err != nil {
			return c.String(500, err.Error())
		}

		return c.String(200, "from ninja: "+dictResponse.Defenition)
	}

	return c.String(200, "from redis: "+result)
}
