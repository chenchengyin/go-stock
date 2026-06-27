package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"math"
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 数据库模型
// ---------------------------------------------------------------------------

// StrategyUser 用户积分信息
type StrategyUser struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    string    `gorm:"uniqueIndex;size:64" json:"userId"` // 与 AppUser.id 对应
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Avatar    string    `gorm:"size:255" json:"avatar"`
	Points    int64     `json:"points"`   // 当前积分
	TotalIn   int64     `json:"totalIn"`  // 累计获得
	TotalOut  int64     `json:"totalOut"` // 累计消耗
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (StrategyUser) TableName() string { return "strategy_users" }

// StrategyPost 帖子
type StrategyPost struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	UserID     string         `gorm:"index;size:64" json:"userId"`
	Nickname   string         `gorm:"size:50" json:"nickname"`
	Title      string         `gorm:"size:200" json:"title"`
	Content    string         `gorm:"type:text" json:"content"`
	Images     StringSlice    `gorm:"type:text" json:"images"`     // JSON 数组 ["url1","url2"]
	LikeCount  int64          `gorm:"default:0" json:"likeCount"`  // 点赞数
	ViewCount  int64          `gorm:"default:0" json:"viewCount"`  // 查看数（去重）
	ViewerIDs  StringSlice    `gorm:"type:text" json:"viewerIds"`  // 已查看的用户id列表
	CommentCnt int64          `gorm:"default:0" json:"commentCnt"` // 评论数
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (StrategyPost) TableName() string { return "strategy_posts" }

// StrategyComment 评论
type StrategyComment struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	PostID      uint           `gorm:"index;not null" json:"postId"`
	ParentID    *uint          `json:"parentId,omitempty"` // nil=直接评论，非nil=回复某条评论
	ReplyToUID  *string        `json:"replyToUid,omitempty"`
	ReplyToName *string        `json:"replyToName,omitempty"`
	UserID      string         `gorm:"size:64;not null" json:"userId"`
	Nickname    string         `gorm:"size:50" json:"nickname"`
	Content     string         `gorm:"type:text" json:"content"`
	Images      StringSlice    `gorm:"type:text" json:"images"`
	CreatedAt   time.Time      `json:"createdAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (StrategyComment) TableName() string { return "strategy_comments" }

// StrategyLike 点赞
type StrategyLike struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	PostID    uint      `gorm:"uniqueIndex:idx_like_post_user;not null" json:"postId"`
	UserID    string    `gorm:"uniqueIndex:idx_like_post_user;size:64;not null" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

func (StrategyLike) TableName() string { return "strategy_likes" }

// StrategyCheckIn 签到
type StrategyCheckIn struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    string    `gorm:"index;size:64;not null" json:"userId"`
	Date      string    `gorm:"index;size:10;not null" json:"date"` // 2024-01-01
	Points    int64     `json:"points"`
	CreatedAt time.Time `json:"createdAt"`
}

func (StrategyCheckIn) TableName() string { return "strategy_checkins" }

// StrategyPointsLog 积分流水
type StrategyPointsLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    string    `gorm:"index;size:64;not null" json:"userId"`
	Delta     int64     `json:"delta"`                          // 正=增加，负=减少
	Remain    int64     `json:"remain"`                         // 变动后剩余
	Reason    string    `gorm:"size:50;not null" json:"reason"` // signin/view_post/reply/like/delete_reply
	RefID     string    `gorm:"size:64" json:"refId"`           // 关联ID（postId/commentId等）
	CreatedAt time.Time `json:"createdAt"`
}

func (StrategyPointsLog) TableName() string { return "strategy_points_logs" }

// StringSlice 存储 JSON 字符串数组
type StringSlice []string

func (s StringSlice) Value() (interface{}, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*s = StringSlice{}
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// ---------------------------------------------------------------------------
// 自动建表
// ---------------------------------------------------------------------------

func autoMigrateStrategy() {
	db.Dao.AutoMigrate(
		&StrategyUser{},
		&StrategyPost{},
		&StrategyComment{},
		&StrategyLike{},
		&StrategyCheckIn{},
		&StrategyPointsLog{},
	)
}

// ---------------------------------------------------------------------------
// StrategyAPI 业务逻辑
// ---------------------------------------------------------------------------

type StrategyAPI struct{}

func NewStrategyAPI() *StrategyAPI {
	return &StrategyAPI{}
}

// ensureUser 确保用户记录存在，不存在则创建（赠送 10 积分）
func (s *StrategyAPI) ensureUser(userID, nickname string) *StrategyUser {
	var u StrategyUser
	err := db.Dao.Where("user_id = ?", userID).First(&u).Error
	if err == nil {
		// 更新昵称
		if u.Nickname != nickname && nickname != "" {
			db.Dao.Model(&u).Update("nickname", nickname)
			u.Nickname = nickname
		}
		return &u
	}
	// 新用户
	u = StrategyUser{
		UserID:   userID,
		Nickname: nickname,
		Points:   10,
		TotalIn:  10,
	}
	db.Dao.Create(&u)
	// 记录流水
	db.Dao.Create(&StrategyPointsLog{
		UserID: userID,
		Delta:  10,
		Remain: 10,
		Reason: "signup_bonus",
	})
	return &u
}

// GetUserPoints 获取用户积分
func (s *StrategyAPI) GetUserPoints(userID string) (*StrategyUser, error) {
	var u StrategyUser
	err := db.Dao.Where("user_id = ?", userID).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CheckIn 签到：每天得 3 分，一天只能一次
func (s *StrategyAPI) CheckIn(userID, nickname string) (*StrategyUser, bool, error) {
	s.ensureUser(userID, nickname)
	today := time.Now().Format("2006-01-02")

	var count int64
	db.Dao.Model(&StrategyCheckIn{}).Where("user_id = ? AND date = ?", userID, today).Count(&count)
	if count > 0 {
		u, _ := s.GetUserPoints(userID)
		return u, false, nil // 已经签到
	}

	// 签到
	db.Dao.Create(&StrategyCheckIn{
		UserID: userID,
		Date:   today,
		Points: 3,
	})

	// 加积分
	var u StrategyUser
	db.Dao.Where("user_id = ?", userID).First(&u)
	u.Points += 3
	u.TotalIn += 3
	db.Dao.Model(&u).Updates(map[string]interface{}{
		"points":   u.Points,
		"total_in": u.TotalIn,
	})

	db.Dao.Create(&StrategyPointsLog{
		UserID: userID,
		Delta:  3,
		Remain: u.Points,
		Reason: "signin",
	})

	return &u, true, nil
}

// CreatePost 发帖
func (s *StrategyAPI) CreatePost(userID, nickname, title, content string, images []string) (*StrategyPost, error) {
	s.ensureUser(userID, nickname)
	imgs := StringSlice(images)
	if imgs == nil {
		imgs = StringSlice{}
	}
	post := StrategyPost{
		UserID:   userID,
		Nickname: nickname,
		Title:    title,
		Content:  content,
		Images:   imgs,
	}
	err := db.Dao.Create(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// GetPosts 获取帖子列表（按时间倒序）
func (s *StrategyAPI) GetPosts(page, pageSize int) ([]*StrategyPost, int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	var total int64
	var posts []*StrategyPost

	db.Dao.Model(&StrategyPost{}).Count(&total)
	db.Dao.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&posts)

	if posts == nil {
		posts = []*StrategyPost{}
	}
	return posts, total
}

// GetPostDetail 获取帖子详情（扣分逻辑在控制器层处理）
func (s *StrategyAPI) GetPostDetail(postID uint) (*StrategyPost, error) {
	var post StrategyPost
	err := db.Dao.First(&post, postID).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// ViewPost 查看帖子（扣分+标记已读）
// viewerIsAuthor: 是否作者本人
// 返回值：(帖子, 是否扣分, 扣分后剩余积分, 错误)
func (s *StrategyAPI) ViewPost(postID uint, viewerID, viewerNickname string) (*StrategyPost, bool, int64, error) {
	post, err := s.GetPostDetail(postID)
	if err != nil {
		return nil, false, 0, err
	}

	// 查看自己的帖子不扣分
	if post.UserID == viewerID {
		return post, false, 0, nil
	}

	// 检查是否首次查看
	viewerSet := make(map[string]bool)
	for _, vid := range post.ViewerIDs {
		viewerSet[vid] = true
	}
	if viewerSet[viewerID] {
		// 已经看过，不扣分
		return post, false, 0, nil
	}

	// 检查积分
	u := s.ensureUser(viewerID, viewerNickname)
	if u.Points <= 0 {
		return nil, false, 0, fmt.Errorf("积分不足，无法查看帖子")
	}

	// 扣分：-1
	u.Points -= 1
	u.TotalOut += 1
	db.Dao.Model(u).Updates(map[string]interface{}{
		"points":    u.Points,
		"total_out": u.TotalOut,
	})

	// 更新查看记录
	post.ViewerIDs = append(post.ViewerIDs, viewerID)
	post.ViewCount++
	viewerIDsJSON, _ := json.Marshal(post.ViewerIDs)
	db.Dao.Model(post).Updates(map[string]interface{}{
		"view_count": post.ViewCount,
		"viewer_ids": string(viewerIDsJSON),
	})

	// 记录流水
	db.Dao.Create(&StrategyPointsLog{
		UserID: viewerID,
		Delta:  -1,
		Remain: u.Points,
		Reason: "view_post",
		RefID:  fmt.Sprintf("%d", postID),
	})

	return post, true, u.Points, nil
}

// ToggleLike 点赞/取消点赞
func (s *StrategyAPI) ToggleLike(postID uint, userID string) (bool, int64, error) {
	var existing StrategyLike
	err := db.Dao.Where("post_id = ? AND user_id = ?", postID, userID).First(&existing).Error
	if err == nil {
		// 已点赞，取消
		db.Dao.Delete(&existing)
		db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("like_count", gorm.Expr("like_count - 1"))
		var post StrategyPost
		db.Dao.First(&post, postID)
		return false, post.LikeCount, nil
	}

	// 点赞
	db.Dao.Create(&StrategyLike{
		PostID: postID,
		UserID: userID,
	})
	db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("like_count", gorm.Expr("like_count + 1"))
	var post StrategyPost
	db.Dao.First(&post, postID)
	return true, post.LikeCount, nil
}

// CreateComment 发表评论/回复
// content: 评论文字
// images: 图片列表（可选）
// parentID: 非nil表示回复某条评论
// 返回值：(评论, 是否加分, 加分后剩余积分, 错误)
func (s *StrategyAPI) CreateComment(postID uint, parentID *uint, userID, nickname, content string, images []string, replyToUID, replyToName *string) (*StrategyComment, bool, int64, error) {
	s.ensureUser(userID, nickname)

	imgs := StringSlice(images)
	if imgs == nil {
		imgs = StringSlice{}
	}

	comment := StrategyComment{
		PostID:      postID,
		ParentID:    parentID,
		UserID:      userID,
		Nickname:    nickname,
		Content:     content,
		Images:      imgs,
		ReplyToUID:  replyToUID,
		ReplyToName: replyToName,
	}
	err := db.Dao.Create(&comment).Error
	if err != nil {
		return nil, false, 0, err
	}

	// 更新评论数
	db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("comment_cnt", gorm.Expr("comment_cnt + 1"))

	addedPoints := int64(0)
	remain := int64(0)

	// 加分规则：回复别人的帖子/评论超过10字或带图片，且不是自己回复自己
	if userID != "" {
		// 获取发帖人
		var post StrategyPost
		db.Dao.First(&post, postID)

		// 不是回复自己的帖子
		if post.UserID != userID {
			hasImage := len(images) > 0
			textLen := len([]rune(content))
			if textLen > 10 || hasImage {
				// 检查今天已经通过回复获得了多少分
				today := time.Now().Format("2006-01-02")
				var todayReplyPoints int64
				db.Dao.Model(&StrategyPointsLog{}).
					Where("user_id = ? AND reason = 'reply' AND created_at >= ? AND created_at < ?",
						userID, today+" 00:00:00", today+" 23:59:59").
					Select("COALESCE(SUM(delta), 0)").
					Scan(&todayReplyPoints)

				if todayReplyPoints < 10 {
					// 加 1 分
					var u StrategyUser
					db.Dao.Where("user_id = ?", userID).First(&u)
					u.Points += 1
					u.TotalIn += 1
					db.Dao.Model(&u).Updates(map[string]interface{}{
						"points":   u.Points,
						"total_in": u.TotalIn,
					})
					addedPoints = 1
					remain = u.Points

					db.Dao.Create(&StrategyPointsLog{
						UserID: userID,
						Delta:  1,
						Remain: u.Points,
						Reason: "reply",
						RefID:  fmt.Sprintf("%d", comment.ID),
					})
				} else {
					var u StrategyUser
					db.Dao.Where("user_id = ?", userID).First(&u)
					remain = u.Points
				}
			} else {
				var u StrategyUser
				db.Dao.Where("user_id = ?", userID).First(&u)
				remain = u.Points
			}
		} else {
			var u StrategyUser
			db.Dao.Where("user_id = ?", userID).First(&u)
			remain = u.Points
		}
	}

	return &comment, addedPoints > 0, remain, nil
}

// GetComments 获取帖子评论（按时间正序）
func (s *StrategyAPI) GetComments(postID uint) ([]*StrategyComment, error) {
	var comments []*StrategyComment
	err := db.Dao.Where("post_id = ?", postID).
		Order("created_at ASC").
		Find(&comments).Error
	if err != nil {
		return nil, err
	}
	if comments == nil {
		comments = []*StrategyComment{}
	}
	return comments, nil
}

// DeleteComment 删除评论（积分回滚）
func (s *StrategyAPI) DeleteComment(commentID uint, userID string) error {
	var comment StrategyComment
	err := db.Dao.First(&comment, commentID).Error
	if err != nil {
		return err
	}
	if comment.UserID != userID {
		return fmt.Errorf("只能删除自己的评论")
	}

	postID := comment.PostID
	db.Dao.Delete(&comment)

	// 评论数减一
	db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("comment_cnt", gorm.Expr("comment_cnt - 1"))

	// 积分回滚：查找这条评论带来的加分记录
	var logs []StrategyPointsLog
	db.Dao.Where("ref_id = ? AND reason = ?", fmt.Sprintf("%d", comment.ID), "reply").Find(&logs)
	for _, log := range logs {
		if log.UserID == userID {
			// 回滚
			var u StrategyUser
			db.Dao.Where("user_id = ?", userID).First(&u)
			u.Points = int64(math.Max(0, float64(u.Points-log.Delta)))
			u.TotalOut += log.Delta
			db.Dao.Model(&u).Updates(map[string]interface{}{
				"points":    u.Points,
				"total_out": u.TotalOut,
			})
			logger.SugaredLogger.Infof("积分回滚: user=%s, delta=%d, remain=%d", userID, log.Delta, u.Points)
		}
	}
	// 删除对应日志
	db.Dao.Where("ref_id = ? AND reason = ?", fmt.Sprintf("%d", comment.ID), "reply").Delete(&StrategyPointsLog{})

	return nil
}

// DeletePost 删除帖子
func (s *StrategyAPI) DeletePost(postID uint, userID string) error {
	var post StrategyPost
	err := db.Dao.First(&post, postID).Error
	if err != nil {
		return err
	}
	if post.UserID != userID {
		return fmt.Errorf("只能删除自己的帖子")
	}

	// 软删除帖子
	db.Dao.Delete(&post)
	// 删除相关评论
	db.Dao.Where("post_id = ?", postID).Delete(&StrategyComment{})
	// 删除相关点赞
	db.Dao.Where("post_id = ?", postID).Delete(&StrategyLike{})

	return nil
}

// GetTodayReplyPoints 获取今日回复已获得积分
func (s *StrategyAPI) GetTodayReplyPoints(userID string) int64 {
	today := time.Now().Format("2006-01-02")
	var total int64
	db.Dao.Model(&StrategyPointsLog{}).
		Where("user_id = ? AND reason = 'reply' AND created_at >= ? AND created_at < ?",
			userID, today+" 00:00:00", today+" 23:59:59").
		Select("COALESCE(SUM(delta), 0)").
		Scan(&total)
	return total
}

// HasCheckedIn 判断今天是否已签到
func (s *StrategyAPI) HasCheckedIn(userID string) bool {
	today := time.Now().Format("2006-01-02")
	var count int64
	db.Dao.Model(&StrategyCheckIn{}).Where("user_id = ? AND date = ?", userID, today).Count(&count)
	return count > 0
}

// HasLiked 判断是否已点赞
func (s *StrategyAPI) HasLiked(postID uint, userID string) bool {
	var count int64
	db.Dao.Model(&StrategyLike{}).Where("post_id = ? AND user_id = ?", postID, userID).Count(&count)
	return count > 0
}

// HasViewed 判断是否已查看
func (s *StrategyAPI) HasViewed(postID uint, userID string) bool {
	var post StrategyPost
	err := db.Dao.First(&post, postID).Error
	if err != nil {
		return false
	}
	for _, vid := range post.ViewerIDs {
		if vid == userID {
			return true
		}
	}
	return false
}
