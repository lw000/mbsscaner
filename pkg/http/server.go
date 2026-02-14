package http

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	logger *zap.Logger
	addr   string
	server *http.Server
	wg     sync.WaitGroup
	stopCh chan struct{}
}

type WritePointRequest struct {
	Name  string      `json:"name" binding:"required"`
	Value interface{} `json:"value" binding:"required"`
}

type WritePointResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type PointWriter interface {
	WritePoint(name string, value interface{}) error
	GetPointValue(name string) (interface{}, error)
}

var pointWriter PointWriter

func RegisterPointWriter(writer PointWriter) {
	pointWriter = writer
}

func NewServer(logger *zap.Logger, addr string) *Server {
	if addr == "" {
		addr = ":8080"
	}

	return &Server{
		logger: logger,
		addr:   addr,
		stopCh: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	router := s.setupRoutes()

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: router,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("HTTP server starting", zap.String("addr", s.addr))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) setupRoutes() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	api := router.Group("/api/v1")
	{
		api.POST("/write-point", s.handleWritePoint)
		api.GET("/read-point/:name", s.handleReadPoint)
		api.GET("/points", s.handleListPoints)
	}

	return router
}

func (s *Server) handleWritePoint(c *gin.Context) {
	if pointWriter == nil {
		c.JSON(http.StatusServiceUnavailable, WritePointResponse{
			Success: false,
			Message: "point writer not registered",
		})
		return
	}

	var req WritePointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, WritePointResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	s.logger.Info("writing point", zap.String("name", req.Name), zap.Any("value", req.Value))

	if err := pointWriter.WritePoint(req.Name, req.Value); err != nil {
		s.logger.Error("failed to write point", zap.Error(err), zap.String("name", req.Name))
		c.JSON(http.StatusInternalServerError, WritePointResponse{
			Success: false,
			Message: fmt.Sprintf("failed to write point: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, WritePointResponse{
		Success: true,
		Message: "point written successfully",
		Data: gin.H{
			"name":  req.Name,
			"value": req.Value,
		},
	})
}

func (s *Server) handleReadPoint(c *gin.Context) {
	if pointWriter == nil {
		c.JSON(http.StatusServiceUnavailable, WritePointResponse{
			Success: false,
			Message: "point writer not registered",
		})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, WritePointResponse{
			Success: false,
			Message: "point name is required",
		})
		return
	}

	value, err := pointWriter.GetPointValue(name)
	if err != nil {
		c.JSON(http.StatusNotFound, WritePointResponse{
			Success: false,
			Message: fmt.Sprintf("point not found: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, WritePointResponse{
		Success: true,
		Data: gin.H{
			"name":  name,
			"value": value,
		},
	})
}

func (s *Server) handleListPoints(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "point list endpoint",
	})
}

func (s *Server) Stop() error {
	close(s.stopCh)

	if s.server != nil {
		if err := s.server.Close(); err != nil {
			return err
		}
	}

	s.wg.Wait()
	s.logger.Info("HTTP server stopped")
	return nil
}
