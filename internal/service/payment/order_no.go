package payment

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func NewOrderNo() string {
	return fmt.Sprintf("TH%s%s", time.Now().UTC().Format("20060102150405"), strings.ReplaceAll(uuid.New().String()[:8], "-", ""))
}
