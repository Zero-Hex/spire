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

type CustomAchievementObjectiveController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomAchievementObjectiveController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomAchievementObjectiveController {
	return &CustomAchievementObjectiveController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomAchievementObjectiveController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_achievement_objective/:id", e.getCustomAchievementObjective, nil),
		routes.RegisterRoute(http.MethodGet, "custom_achievement_objectives", e.listCustomAchievementObjectives, nil),
		routes.RegisterRoute(http.MethodGet, "custom_achievement_objectives/count", e.getCustomAchievementObjectivesCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_achievement_objective", e.createCustomAchievementObjective, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_achievement_objective/:id", e.deleteCustomAchievementObjective, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_achievement_objective/:id", e.updateCustomAchievementObjective, nil),
		routes.RegisterRoute(http.MethodPost, "custom_achievement_objectives/bulk", e.getCustomAchievementObjectivesBulk, nil),
	}
}

// listCustomAchievementObjectives godoc
// @Id listCustomAchievementObjectives
// @Summary Lists CustomAchievementObjectives
// @Accept json
// @Produce json
// @Tags CustomAchievementObjective
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_objectives [get]
func (e *CustomAchievementObjectiveController) listCustomAchievementObjectives(c echo.Context) error {
	var results []models.CustomAchievementObjective
	err := e.db.QueryContext(models.CustomAchievementObjective{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomAchievementObjective godoc
// @Id getCustomAchievementObjective
// @Summary Gets CustomAchievementObjective
// @Accept json
// @Produce json
// @Tags CustomAchievementObjective
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_objective/{id} [get]
func (e *CustomAchievementObjectiveController) getCustomAchievementObjective(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomAchievementObjective
	query := e.db.QueryContext(models.CustomAchievementObjective{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.ID == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateCustomAchievementObjective godoc
// @Id updateCustomAchievementObjective
// @Summary Updates CustomAchievementObjective
// @Accept json
// @Produce json
// @Tags CustomAchievementObjective
// @Param id path int true "Id"
// @Param custom_achievement_objective body models.CustomAchievementObjective true "CustomAchievementObjective"
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_achievement_objective/{id} [patch]
func (e *CustomAchievementObjectiveController) updateCustomAchievementObjective(c echo.Context) error {
	request := new(models.CustomAchievementObjective)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomAchievementObjective
	query := e.db.QueryContext(models.CustomAchievementObjective{}, c)
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
		event := fmt.Sprintf("Updated [CustomAchievementObjective] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomAchievementObjective godoc
// @Id createCustomAchievementObjective
// @Summary Creates CustomAchievementObjective
// @Accept json
// @Produce json
// @Param custom_achievement_objective body models.CustomAchievementObjective true "CustomAchievementObjective"
// @Tags CustomAchievementObjective
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_achievement_objective [put]
func (e *CustomAchievementObjectiveController) createCustomAchievementObjective(c echo.Context) error {
	customAchievementObjective := new(models.CustomAchievementObjective)
	if err := c.Bind(customAchievementObjective); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomAchievementObjective{}, c).Model(&models.CustomAchievementObjective{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customAchievementObjective).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomAchievementObjective{}, customAchievementObjective)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomAchievementObjective] [%v] data [%v]", customAchievementObjective.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customAchievementObjective)
}

// deleteCustomAchievementObjective godoc
// @Id deleteCustomAchievementObjective
// @Summary Deletes CustomAchievementObjective
// @Accept json
// @Produce json
// @Tags CustomAchievementObjective
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_achievement_objective/{id} [delete]
func (e *CustomAchievementObjectiveController) deleteCustomAchievementObjective(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomAchievementObjective
	query := e.db.QueryContext(models.CustomAchievementObjective{}, c)
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
		event := fmt.Sprintf("Deleted [CustomAchievementObjective] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomAchievementObjectivesBulk godoc
// @Id getCustomAchievementObjectivesBulk
// @Summary Gets CustomAchievementObjectives in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomAchievementObjective
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_objectives/bulk [post]
func (e *CustomAchievementObjectiveController) getCustomAchievementObjectivesBulk(c echo.Context) error {
	var results []models.CustomAchievementObjective

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

	err := e.db.QueryContext(models.CustomAchievementObjective{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomAchievementObjectivesCount godoc
// @Id getCustomAchievementObjectivesCount
// @Summary Counts CustomAchievementObjectives
// @Accept json
// @Produce json
// @Tags CustomAchievementObjective
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementObjective
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_objectives/count [get]
func (e *CustomAchievementObjectiveController) getCustomAchievementObjectivesCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomAchievementObjective{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}