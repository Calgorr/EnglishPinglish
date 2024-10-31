package handlers

import (
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
