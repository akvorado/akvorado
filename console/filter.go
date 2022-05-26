package console

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/console/filter"
)

// filterValidateHandlerInput describes the input for the /filter/validate endpoint.
type filterValidateHandlerInput struct {
	Filter string `json:"filter"`
}

// filterValidateHandlerOutput describes the output for the /filter/validate endpoint.
type filterValidateHandlerOutput struct {
	Message string        `json:"message"`
	Parsed  string        `json:"parsed,omitempty"`
	Errors  filter.Errors `json:"errors,omitempty"`
}

func (c *Component) filterValidateHandlerFunc(gc *gin.Context) {
	var input filterValidateHandlerInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	got, err := filter.Parse("", []byte(input.Filter))
	if err == nil {
		gc.JSON(http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
			Parsed:  got.(string),
		})
		return
	}
	gc.JSON(http.StatusBadRequest, filterValidateHandlerOutput{
		Message: filter.HumanError(err),
		Errors:  filter.AllErrors(err),
	})
}
