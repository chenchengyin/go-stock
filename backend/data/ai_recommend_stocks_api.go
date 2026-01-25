// Package data ai_recommend_stocks_api.go
package data

import (
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/strutil"
)

type AiRecommendStocksService struct{}

func NewAiRecommendStocksService() *AiRecommendStocksService {
	return &AiRecommendStocksService{}
}

// CreateAiRecommendStocks 创建AI推荐股票记录
func (s *AiRecommendStocksService) CreateAiRecommendStocks(recommend *models.AiRecommendStocks) error {
	result := db.Dao.Create(recommend)
	return result.Error
}

// GetAiRecommendStocksList 分页查询AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksList(query *models.AiRecommendStocksQuery) (*models.AiRecommendStocksPageData, error) {
	var list []models.AiRecommendStocks
	var total int64

	q := db.Dao.Model(&models.AiRecommendStocks{})

	// 构建查询条件
	if query.StockCode != "" {
		q.Or("stock_code LIKE ?", "%"+query.StockCode+"%")
	}
	if query.StockName != "" {
		q.Or("stock_name LIKE ?", "%"+query.StockName+"%")
	}
	if query.BkCode != "" {
		q.Or("bk_code LIKE ?", "%"+query.BkCode+"%")
	}
	if query.BkName != "" {
		q.Or("bk_name LIKE ?", "%"+query.BkName+"%")
	}
	if query.StartDate != "" && query.EndDate != "" {
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		q = q.Where("data_time BETWEEN ? AND ?", query.StartDate, query.EndDate)
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.AiRecommendStocksPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAiRecommendStocksByID 根据ID获取AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksByID(id uint) (*models.AiRecommendStocks, error) {
	var recommend models.AiRecommendStocks
	err := db.Dao.First(&recommend, id).Error
	if err != nil {
		return nil, err
	}
	return &recommend, nil
}

// UpdateAiRecommendStocks 更新AI推荐股票记录
func (s *AiRecommendStocksService) UpdateAiRecommendStocks(id uint, recommend *models.AiRecommendStocks) error {
	result := db.Dao.Model(&models.AiRecommendStocks{}).Where("id = ?", id).Updates(recommend)
	return result.Error
}

// DeleteAiRecommendStocks 根据ID删除AI推荐股票记录
func (s *AiRecommendStocksService) DeleteAiRecommendStocks(id uint) error {
	// 使用软删除
	result := db.Dao.Where("id = ?", id).Delete(&models.AiRecommendStocks{})
	return result.Error
}

// BatchDeleteAiRecommendStocks 批量删除AI推荐股票记录
func (s *AiRecommendStocksService) BatchDeleteAiRecommendStocks(ids []uint) error {
	// 使用软删除
	result := db.Dao.Where("id IN ?", ids).Delete(&models.AiRecommendStocks{})
	return result.Error
}
