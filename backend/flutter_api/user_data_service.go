package flutter_api

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-stock/backend/data"

	"github.com/duke-git/lancet/v2/convertor"
	"gorm.io/gorm"
)

var userDataQuoteFetcher = func(stockCodes ...string) (*[]data.StockInfo, error) {
	return data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
}

type UserDataService struct {
	dao *gorm.DB
}

func NewUserDataService(dao *gorm.DB) *UserDataService {
	return &UserDataService{dao: dao}
}

func (s *UserDataService) ListFollowedStocks(ctx context.Context, userID string, groupID uint) ([]data.FollowedStock, error) {
	if groupID != 0 {
		owns, err := s.OwnsGroup(ctx, userID, groupID)
		if err != nil {
			return nil, err
		}
		if !owns {
			return []data.FollowedStock{}, nil
		}

		var codes []string
		err = s.dao.WithContext(ctx).
			Model(&data.GroupStock{}).
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Pluck("stock_code", &codes).Error
		if err != nil {
			return nil, err
		}
		if len(codes) == 0 {
			return []data.FollowedStock{}, nil
		}

		var items []data.FollowedStock
		err = s.dao.WithContext(ctx).
			Model(&data.FollowedStock{}).
			Where("user_id = ? AND stock_code IN ? AND is_del = ?", userID, codes, 0).
			Order("sort ASC, time DESC").
			Find(&items).Error
		return items, err
	}

	var items []data.FollowedStock
	err := s.dao.WithContext(ctx).
		Model(&data.FollowedStock{}).
		Where("user_id = ? AND is_del = ?", userID, 0).
		Order("sort ASC, time DESC").
		Find(&items).Error
	return items, err
}

func (s *UserDataService) ListGroups(ctx context.Context, userID string) ([]data.Group, error) {
	groups := []data.Group{}
	err := s.dao.WithContext(ctx).
		Model(&data.Group{}).
		Where("user_id = ?", userID).
		Order("sort ASC, id ASC").
		Find(&groups).Error
	return groups, err
}

func (s *UserDataService) ListGroupStocks(ctx context.Context, userID string, groupID uint) ([]data.GroupStock, error) {
	owns, err := s.OwnsGroup(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return []data.GroupStock{}, nil
	}

	stocks := []data.GroupStock{}
	err = s.dao.WithContext(ctx).
		Model(&data.GroupStock{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Order("id ASC").
		Find(&stocks).Error
	return stocks, err
}

func (s *UserDataService) ListTradingRecords(ctx context.Context, userID string) ([]data.TradingRecord, error) {
	records := []data.TradingRecord{}
	err := s.dao.WithContext(ctx).
		Model(&data.TradingRecord{}).
		Where("user_id = ?", userID).
		Order("trading_time DESC, id DESC").
		Find(&records).Error
	return records, err
}

func (s *UserDataService) Follow(ctx context.Context, userID, stockCode string) (string, error) {
	normalizedCode := normalizeUserOwnedStockCode(stockCode)
	if normalizedCode == "" {
		return "关注失败", nil
	}

	var existing data.FollowedStock
	err := s.dao.WithContext(ctx).
		Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ? AND is_del = ?", userID, normalizedCode, 0).
		First(&existing).Error
	if err == nil {
		return "已经关注了", nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "关注失败", err
	}

	var activeCount int64
	if err := s.dao.WithContext(ctx).
		Model(&data.FollowedStock{}).
		Where("user_id = ? AND is_del = ?", userID, 0).
		Count(&activeCount).Error; err != nil {
		return "关注失败", err
	}
	if activeCount >= 63 {
		return "最多只能关注63只股票", nil
	}

	stockInfos, err := userDataQuoteFetcher(normalizedCode)
	if err != nil || stockInfos == nil || len(*stockInfos) == 0 {
		return "关注失败", err
	}

	maxSort := int64(0)
	if err := s.dao.WithContext(ctx).
		Model(&data.FollowedStock{}).
		Select("COALESCE(MAX(sort), 0)").
		Where("user_id = ?", userID).
		Scan(&maxSort).Error; err != nil {
		return "关注失败", err
	}

	stockInfo := (*stockInfos)[0]
	price, _ := convertor.ToFloat(stockInfo.Price)
	ownedUserID := userID
	updates := map[string]any{
		"user_id":              ownedUserID,
		"stock_code":           normalizedCode,
		"name":                 stockInfo.Name,
		"price":                price,
		"time":                 time.Now(),
		"change_percent":       0,
		"price_change":         0,
		"alarm_change_percent": 3,
		"alarm_price":          price + 1,
		"is_del":               0,
	}

	var deleted data.FollowedStock
	err = s.dao.WithContext(ctx).
		Unscoped().
		Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ?", userID, normalizedCode).
		First(&deleted).Error
	if err == nil {
		updates["sort"] = deleted.Sort
		if deleted.Sort == 0 {
			updates["sort"] = maxSort + 1
		}
		err = s.dao.WithContext(ctx).
			Unscoped().
			Model(&data.FollowedStock{}).
			Where("user_id = ? AND stock_code = ?", userID, normalizedCode).
			Updates(updates).Error
		if err != nil {
			return "关注失败", err
		}
		return "关注成功", nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "关注失败", err
	}

	updates["sort"] = maxSort + 1
	record := data.FollowedStock{
		UserID:             &ownedUserID,
		StockCode:          normalizedCode,
		Name:               stockInfo.Name,
		Price:              price,
		Time:               updates["time"].(time.Time),
		ChangePercent:      0,
		PriceChange:        0,
		Sort:               maxSort + 1,
		AlarmChangePercent: 3,
		AlarmPrice:         price + 1,
	}
	if err := s.dao.WithContext(ctx).Create(&record).Error; err != nil {
		if isFollowedStockUniqueViolation(err) {
			return "已经关注了", nil
		}
		return "关注失败", err
	}

	return "关注成功", nil
}

func (s *UserDataService) Unfollow(ctx context.Context, userID, stockCode string) (string, error) {
	normalizedCode := normalizeUserOwnedStockCode(stockCode)
	if normalizedCode == "" {
		return "未找到当前用户记录", nil
	}

	result := s.dao.WithContext(ctx).
		Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ? AND is_del = ?", userID, normalizedCode, 0).
		Delete(&data.FollowedStock{})
	if result.Error != nil {
		return "取消关注失败", result.Error
	}
	if result.RowsAffected == 0 {
		return "未找到当前用户记录", nil
	}
	return "取消关注成功", nil
}

func (s *UserDataService) OwnsGroup(ctx context.Context, userID string, groupID uint) (bool, error) {
	var count int64
	err := s.dao.WithContext(ctx).
		Model(&data.Group{}).
		Where("id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error
	return count > 0, err
}

func normalizeUserOwnedStockCode(stockCode string) string {
	code := strings.TrimSpace(stockCode)
	if code == "" {
		return ""
	}

	lower := strings.ToLower(code)
	switch {
	case strings.HasSuffix(lower, ".sz"):
		return "sz" + strings.TrimSuffix(lower, ".sz")
	case strings.HasSuffix(lower, ".sh"):
		return "sh" + strings.TrimSuffix(lower, ".sh")
	case strings.HasSuffix(lower, ".hk"):
		return "hk" + strings.TrimSuffix(lower, ".hk")
	case strings.HasSuffix(lower, ".bj"):
		return "bj" + strings.TrimSuffix(lower, ".bj")
	case strings.HasPrefix(lower, "us"):
		return "gb_" + strings.TrimPrefix(lower, "us")
	case len(lower) == 6 && isDigitsOnly(lower):
		switch lower[0] {
		case '6':
			return "sh" + lower
		case '0', '3':
			return "sz" + lower
		case '4', '8':
			return "bj" + lower
		}
	}

	return lower
}

func isDigitsOnly(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isFollowedStockUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "idx_followed_stock_user_code") || strings.Contains(text, "UNIQUE constraint failed")
}
