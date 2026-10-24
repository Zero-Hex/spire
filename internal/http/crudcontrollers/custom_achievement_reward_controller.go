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

type CustomAchievementRewardController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomAchievementRewardController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomAchievementRewardController {
	return &CustomAchievementRewardController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomAchievementRewardController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_achievement_reward/:id", e.getCustomAchievementReward, nil),
		routes.RegisterRoute(http.MethodGet, "custom_achievement_rewards", e.listCustomAchievementRewards, nil),
		routes.RegisterRoute(http.MethodGet, "custom_achievement_rewards/count", e.getCustomAchievementRewardsCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_achievement_reward", e.createCustomAchievementReward, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_achievement_reward/:id", e.deleteCustomAchievementReward, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_achievement_reward/:id", e.updateCustomAchievementReward, nil),
		routes.RegisterRoute(http.MethodPost, "custom_achievement_rewards/bulk", e.getCustomAchievementRewardsBulk, nil),
	}
}

// listCustomAchievementRewards godoc
// @Id listCustomAchievementRewards
// @Summary Lists CustomAchievementRewards
// @Accept json
// @Produce json
// @Tags CustomAchievementReward
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementReward
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_rewards [get]
func (e *CustomAchievementRewardController) listCustomAchievementRewards(c echo.Context) error {
	var results []models.CustomAchievementReward
	err := e.db.QueryContext(models.CustomAchievementReward{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomAchievementReward godoc
// @Id getCustomAchievementReward
// @Summary Gets CustomAchievementReward
// @Accept json
// @Produce json
// @Tags CustomAchievementReward
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementReward
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_reward/{id} [get]
func (e *CustomAchievementRewardController) getCustomAchievementReward(c echo.Context) error {
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
	var result models.CustomAchievementReward
	query := e.db.QueryContext(models.CustomAchievementReward{}, c)
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

// updateCustomAchievementReward godoc
// @Id updateCustomAchievementReward
// @Summary Updates CustomAchievementReward
// @Accept json
// @Produce json
// @Tags CustomAchievementReward
// @Param id path int true "Id"
// @Param custom_achievement_reward body models.CustomAchievementReward true "CustomAchievementReward"
// @Success 200 {array} models.CustomAchievementReward
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_achievement_reward/{id} [patch]
func (e *CustomAchievementRewardController) updateCustomAchievementReward(c echo.Context) error {
	request := new(models.CustomAchievementReward)
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
	var result models.CustomAchievementReward
	query := e.db.QueryContext(models.CustomAchievementReward{}, c)
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
		event := fmt.Sprintf("Updated [CustomAchievementReward] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomAchievementReward godoc
// @Id createCustomAchievementReward
// @Summary Creates CustomAchievementReward
// @Accept json
// @Produce json
// @Param custom_achievement_reward body models.CustomAchievementReward true "CustomAchievementReward"
// @Tags CustomAchievementReward
// @Success 200 {array} models.CustomAchievementReward
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_achievement_reward [put]
func (e *CustomAchievementRewardController) createCustomAchievementReward(c echo.Context) error {
	customAchievementReward := new(models.CustomAchievementReward)
	if err := c.Bind(customAchievementReward); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomAchievementReward{}, c).Model(&models.CustomAchievementReward{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customAchievementReward).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomAchievementReward{}, customAchievementReward)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomAchievementReward] [%v] data [%v]", customAchievementReward.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customAchievementReward)
}

// deleteCustomAchievementReward godoc
// @Id deleteCustomAchievementReward
// @Summary Deletes CustomAchievementReward
// @Accept json
// @Produce json
// @Tags CustomAchievementReward
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_achievement_reward/{id} [delete]
func (e *CustomAchievementRewardController) deleteCustomAchievementReward(c echo.Context) error {
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
	var result models.CustomAchievementReward
	query := e.db.QueryContext(models.CustomAchievementReward{}, c)
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
		event := fmt.Sprintf("Deleted [CustomAchievementReward] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomAchievementRewardsBulk godoc
// @Id getCustomAchievementRewardsBulk
// @Summary Gets CustomAchievementRewards in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomAchievementReward
// @Success 200 {array} models.CustomAchievementReward
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_rewards/bulk [post]
func (e *CustomAchievementRewardController) getCustomAchievementRewardsBulk(c echo.Context) error {
	var results []models.CustomAchievementReward

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

	err := e.db.QueryContext(models.CustomAchievementReward{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomAchievementRewardsCount godoc
// @Id getCustomAchievementRewardsCount
// @Summary Counts CustomAchievementRewards
// @Accept json
// @Produce json
// @Tags CustomAchievementReward
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomAchievementReward
// @Failure 500 {string} string "Bad query request"
// @Router /custom_achievement_rewards/count [get]
func (e *CustomAchievementRewardController) getCustomAchievementRewardsCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomAchievementReward{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}