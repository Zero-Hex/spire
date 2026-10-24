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

type FactionWindowPinnedController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewFactionWindowPinnedController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *FactionWindowPinnedController {
	return &FactionWindowPinnedController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *FactionWindowPinnedController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "faction_window_pinned/:factionId", e.getFactionWindowPinned, nil),
		routes.RegisterRoute(http.MethodGet, "faction_window_pinneds", e.listFactionWindowPinneds, nil),
		routes.RegisterRoute(http.MethodGet, "faction_window_pinneds/count", e.getFactionWindowPinnedsCount, nil),
		routes.RegisterRoute(http.MethodPut, "faction_window_pinned", e.createFactionWindowPinned, nil),
		routes.RegisterRoute(http.MethodDelete, "faction_window_pinned/:factionId", e.deleteFactionWindowPinned, nil),
		routes.RegisterRoute(http.MethodPatch, "faction_window_pinned/:factionId", e.updateFactionWindowPinned, nil),
		routes.RegisterRoute(http.MethodPost, "faction_window_pinneds/bulk", e.getFactionWindowPinnedsBulk, nil),
	}
}

// listFactionWindowPinneds godoc
// @Id listFactionWindowPinneds
// @Summary Lists FactionWindowPinneds
// @Accept json
// @Produce json
// @Tags FactionWindowPinned
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.FactionWindowPinned
// @Failure 500 {string} string "Bad query request"
// @Router /faction_window_pinneds [get]
func (e *FactionWindowPinnedController) listFactionWindowPinneds(c echo.Context) error {
	var results []models.FactionWindowPinned
	err := e.db.QueryContext(models.FactionWindowPinned{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getFactionWindowPinned godoc
// @Id getFactionWindowPinned
// @Summary Gets FactionWindowPinned
// @Accept json
// @Produce json
// @Tags FactionWindowPinned
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.FactionWindowPinned
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /faction_window_pinned/{id} [get]
func (e *FactionWindowPinnedController) getFactionWindowPinned(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	factionId, err := strconv.Atoi(c.Param("factionId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [FactionId]"})
	}
	params = append(params, factionId)
	keys = append(keys, "faction_id = ?")

	// query builder
	var result models.FactionWindowPinned
	query := e.db.QueryContext(models.FactionWindowPinned{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.FactionId == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateFactionWindowPinned godoc
// @Id updateFactionWindowPinned
// @Summary Updates FactionWindowPinned
// @Accept json
// @Produce json
// @Tags FactionWindowPinned
// @Param id path int true "Id"
// @Param faction_window_pinned body models.FactionWindowPinned true "FactionWindowPinned"
// @Success 200 {array} models.FactionWindowPinned
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /faction_window_pinned/{id} [patch]
func (e *FactionWindowPinnedController) updateFactionWindowPinned(c echo.Context) error {
	request := new(models.FactionWindowPinned)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	factionId, err := strconv.Atoi(c.Param("factionId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [FactionId]"})
	}
	params = append(params, factionId)
	keys = append(keys, "faction_id = ?")

	// query builder
	var result models.FactionWindowPinned
	query := e.db.QueryContext(models.FactionWindowPinned{}, c)
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
		event := fmt.Sprintf("Updated [FactionWindowPinned] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createFactionWindowPinned godoc
// @Id createFactionWindowPinned
// @Summary Creates FactionWindowPinned
// @Accept json
// @Produce json
// @Param faction_window_pinned body models.FactionWindowPinned true "FactionWindowPinned"
// @Tags FactionWindowPinned
// @Success 200 {array} models.FactionWindowPinned
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /faction_window_pinned [put]
func (e *FactionWindowPinnedController) createFactionWindowPinned(c echo.Context) error {
	factionWindowPinned := new(models.FactionWindowPinned)
	if err := c.Bind(factionWindowPinned); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.FactionWindowPinned{}, c).Model(&models.FactionWindowPinned{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&factionWindowPinned).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.FactionWindowPinned{}, factionWindowPinned)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [FactionWindowPinned] [%v] data [%v]", factionWindowPinned.FactionId, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, factionWindowPinned)
}

// deleteFactionWindowPinned godoc
// @Id deleteFactionWindowPinned
// @Summary Deletes FactionWindowPinned
// @Accept json
// @Produce json
// @Tags FactionWindowPinned
// @Param id path int true "factionId"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /faction_window_pinned/{id} [delete]
func (e *FactionWindowPinnedController) deleteFactionWindowPinned(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	factionId, err := strconv.Atoi(c.Param("factionId"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, factionId)
	keys = append(keys, "faction_id = ?")

	// query builder
	var result models.FactionWindowPinned
	query := e.db.QueryContext(models.FactionWindowPinned{}, c)
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
		event := fmt.Sprintf("Deleted [FactionWindowPinned] [%v] keys [%v]", result.FactionId, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getFactionWindowPinnedsBulk godoc
// @Id getFactionWindowPinnedsBulk
// @Summary Gets FactionWindowPinneds in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags FactionWindowPinned
// @Success 200 {array} models.FactionWindowPinned
// @Failure 500 {string} string "Bad query request"
// @Router /faction_window_pinneds/bulk [post]
func (e *FactionWindowPinnedController) getFactionWindowPinnedsBulk(c echo.Context) error {
	var results []models.FactionWindowPinned

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

	err := e.db.QueryContext(models.FactionWindowPinned{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getFactionWindowPinnedsCount godoc
// @Id getFactionWindowPinnedsCount
// @Summary Counts FactionWindowPinneds
// @Accept json
// @Produce json
// @Tags FactionWindowPinned
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.FactionWindowPinned
// @Failure 500 {string} string "Bad query request"
// @Router /faction_window_pinneds/count [get]
func (e *FactionWindowPinnedController) getFactionWindowPinnedsCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.FactionWindowPinned{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}