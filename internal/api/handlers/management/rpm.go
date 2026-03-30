package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetRPMStats(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusOK, gin.H{"stats": map[string]any{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": h.authManager.RPMStats()})
}
