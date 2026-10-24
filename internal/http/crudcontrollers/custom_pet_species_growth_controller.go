package crudcontrollers

import (
	"fmt"
	"github.com/EQEmu/spire/internal/auditlog"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
)

type CustomPetSpeciesGrowthController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomPetSpeciesGrowthController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomPetSpeciesGrowthController {
	return &CustomPetSpeciesGrowthController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomPetSpeciesGrowthController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_pet_species_growth/:speciesId", e.getCustomPetSpeciesGrowth, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_species_growths", e.listCustomPetSpeciesGrowths, nil),
		routes.RegisterRoute(http.MethodGet, "custom_pet_species_growths/count", e.getCustomPetSpeciesGrowthsCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_pet_species_growth", e.createCustomPetSpeciesGrowth, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_pet_species_growth/:speciesId", e.deleteCustomPetSpeciesGrowth, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_pet_species_growth/:speciesId", e.updateCustomPetSpeciesGrowth, nil),
		routes.RegisterRoute(http.MethodPost, "custom_pet_species_growths/bulk", e.getCustomPetSpeciesGrowthsBulk, nil),
	}
}

// listCustomPetSpeciesGrowths godoc
// @Id listCustomPetSpeciesGrowths
// @Summary Lists CustomPetSpeciesGrowths
// @Accept json
// @Produce json
// @Tags CustomPetSpeciesGrowth
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species_growths [get]
func (e *CustomPetSpeciesGrowthController) listCustomPetSpeciesGrowths(c echo.Context) error {
	var results []models.CustomPetSpeciesGrowth
	err := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetSpeciesGrowth godoc
// @Id getCustomPetSpeciesGrowth
// @Summary Gets CustomPetSpeciesGrowth
// @Accept json
// @Produce json
// @Tags CustomPetSpeciesGrowth
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species_growth/{id} [get]
func (e *CustomPetSpeciesGrowthController) getCustomPetSpeciesGrowth(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	speciesId, err := strconv.Atoi(c.Param("speciesId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [SpeciesId]"})
	}
	params = append(params, speciesId)
	keys = append(keys, "species_id = ?")

	// query builder
	var result models.CustomPetSpeciesGrowth
	query := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.SpeciesId == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateCustomPetSpeciesGrowth godoc
// @Id updateCustomPetSpeciesGrowth
// @Summary Updates CustomPetSpeciesGrowth
// @Accept json
// @Produce json
// @Tags CustomPetSpeciesGrowth
// @Param id path int true "Id"
// @Param custom_pet_species_growth body models.CustomPetSpeciesGrowth true "CustomPetSpeciesGrowth"
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_pet_species_growth/{id} [patch]
func (e *CustomPetSpeciesGrowthController) updateCustomPetSpeciesGrowth(c echo.Context) error {
	request := new(models.CustomPetSpeciesGrowth)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	speciesId, err := strconv.Atoi(c.Param("speciesId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [SpeciesId]"})
	}
	params = append(params, speciesId)
	keys = append(keys, "species_id = ?")

	// query builder
	var result models.CustomPetSpeciesGrowth
	query := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Cannot find entity [%s]", err.Error())})
	}

	// save top-level using only changes
	diff := database.ResultDifference(result, request)
	err = query.Session(&gorm.Session{FullSaveAssociations: false}).Updates(diff).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Error updating entity [%v]", err.Error())})
	}

	// log update event
	if e.db.GetSpireDb() != nil && len(diff) > 0 {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// build fields updated
		var fieldsUpdated []string
		for k, v := range diff {
			fieldsUpdated = append(fieldsUpdated, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Updated [CustomPetSpeciesGrowth] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomPetSpeciesGrowth godoc
// @Id createCustomPetSpeciesGrowth
// @Summary Creates CustomPetSpeciesGrowth
// @Accept json
// @Produce json
// @Param custom_pet_species_growth body models.CustomPetSpeciesGrowth true "CustomPetSpeciesGrowth"
// @Tags CustomPetSpeciesGrowth
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_pet_species_growth [put]
func (e *CustomPetSpeciesGrowthController) createCustomPetSpeciesGrowth(c echo.Context) error {
	customPetSpeciesGrowth := new(models.CustomPetSpeciesGrowth)
	if err := c.Bind(customPetSpeciesGrowth); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomPetSpeciesGrowth{}, c).Model(&models.CustomPetSpeciesGrowth{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customPetSpeciesGrowth).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomPetSpeciesGrowth{}, customPetSpeciesGrowth)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomPetSpeciesGrowth] [%v] data [%v]", customPetSpeciesGrowth.SpeciesId, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customPetSpeciesGrowth)
}

// deleteCustomPetSpeciesGrowth godoc
// @Id deleteCustomPetSpeciesGrowth
// @Summary Deletes CustomPetSpeciesGrowth
// @Accept json
// @Produce json
// @Tags CustomPetSpeciesGrowth
// @Param id path int true "speciesId"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_pet_species_growth/{id} [delete]
func (e *CustomPetSpeciesGrowthController) deleteCustomPetSpeciesGrowth(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	speciesId, err := strconv.Atoi(c.Param("speciesId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, speciesId)
	keys = append(keys, "species_id = ?")

	// query builder
	var result models.CustomPetSpeciesGrowth
	query := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = query.Limit(10000).Delete(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error deleting entity"})
	}

	// log delete event
	if e.db.GetSpireDb() != nil {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// record event
		event := fmt.Sprintf("Deleted [CustomPetSpeciesGrowth] [%v] keys [%v]", result.SpeciesId, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomPetSpeciesGrowthsBulk godoc
// @Id getCustomPetSpeciesGrowthsBulk
// @Summary Gets CustomPetSpeciesGrowths in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomPetSpeciesGrowth
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species_growths/bulk [post]
func (e *CustomPetSpeciesGrowthController) getCustomPetSpeciesGrowthsBulk(c echo.Context) error {
	var results []models.CustomPetSpeciesGrowth

	r := new(BulkFetchByIdsGetRequest)
	if err := c.Bind(r); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to bulk request: [%v]", err.Error())},
		)
	}

	if len(r.IDs) == 0 {
		return c.JSON(
			http.StatusOK,
			echo.Map{"error": fmt.Sprintf("Missing request field data 'ids'")},
		)
	}

	err := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomPetSpeciesGrowthsCount godoc
// @Id getCustomPetSpeciesGrowthsCount
// @Summary Counts CustomPetSpeciesGrowths
// @Accept json
// @Produce json
// @Tags CustomPetSpeciesGrowth
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomPetSpeciesGrowth
// @Failure 500 {string} string "Bad query request"
// @Router /custom_pet_species_growths/count [get]
func (e *CustomPetSpeciesGrowthController) getCustomPetSpeciesGrowthsCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomPetSpeciesGrowth{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}