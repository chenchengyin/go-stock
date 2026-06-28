package flutter_api

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
	UserID    string    `gorm:"uniqueIndex;size:64" json:"userId"`
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Avatar    string    `gorm:"size:255" json:"avatar"`
	Points    int64     `json:"points"`
	TotalIn   int64     `json:"totalIn"`
	TotalOut  int64     `json:"totalOut"`
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
	Images     StringSlice    `gorm:"type:text" json:"images"`
	LikeCount  int64          `gorm:"default:0" json:"likeCount"`
	ViewCount  int64          `gorm:"default:0" json:"viewCount"`
	ViewerIDs  StringSlice    `gorm:"type:text" json:"viewerIds"`
	CommentCnt int64          `gorm:"default:0" json:"commentCnt"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (StrategyPost) TableName() string { return "strategy_posts" }

// StrategyComment 评论
type StrategyComment struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	PostID      uint           `gorm:"index;not null" json:"postId"`
	ParentID    *uint          `json:"parentId,omitempty"`
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
	Date      string    `gorm:"index;size:10;not null" json:"date"`
	Points    int64     `json:"points"`
	CreatedAt time.Time `json:"createdAt"`
}

func (StrategyCheckIn) TableName() string { return "strategy_checkins" }

// StrategyPointsLog 积分流水
type StrategyPointsLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    string    `gorm:"index;size:64;not null" json:"userId"`
	Delta     int64     `json:"delta"`
	Remain    int64     `json:"remain"`
	Reason    string    `gorm:"size:50;not null" json:"reason"`
	RefID     string    `gorm:"size:64" json:"refId"`
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
		if u.Nickname != nickname && nickname != "" {
			db.Dao.Model(&u).Update("nickname", nickname)
			u.Nickname = nickname
		}
		return &u
	}
	u = StrategyUser{
		UserID:   userID,
		Nickname: nickname,
		Points:   10,
		TotalIn:  10,
	}
	db.Dao.Create(&u)
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

// CheckIn 签到：每天得 3 分
func (s *StrategyAPI) CheckIn(userID, nickname string) (*StrategyUser, bool, error) {
	s.ensureUser(userID, nickname)
	today := time.Now().Format("2006-01-02")

	var count int64
	db.Dao.Model(&StrategyCheckIn{}).Where("user_id = ? AND date = ?", userID, today).Count(&count)
	if count > 0 {
		u, _ := s.GetUserPoints(userID)
		return u, false, nil
	}

	db.Dao.Create(&StrategyCheckIn{
		UserID: userID,
		Date:   today,
		Points: 3,
	})

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

// GetPostDetail 获取帖子详情
func (s *StrategyAPI) GetPostDetail(postID uint) (*StrategyPost, error) {
	var post StrategyPost
	err := db.Dao.First(&post, postID).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// ViewPost 查看帖子（扣分+标记已读）
func (s *StrategyAPI) ViewPost(postID uint, viewerID, viewerNickname string) (*StrategyPost, bool, int64, error) {
	post, err := s.GetPostDetail(postID)
	if err != nil {
		return nil, false, 0, err
	}

	if post.UserID == viewerID {
		return post, false, 0, nil
	}

	viewerSet := make(map[string]bool)
	for _, vid := range post.ViewerIDs {
		viewerSet[vid] = true
	}
	if viewerSet[viewerID] {
		return post, false, 0, nil
	}

	u := s.ensureUser(viewerID, viewerNickname)
	if u.Points <= 0 {
		return nil, false, 0, fmt.Errorf("积分不足，无法查看帖子")
	}

	u.Points -= 1
	u.TotalOut += 1
	db.Dao.Model(u).Updates(map[string]interface{}{
		"points":    u.Points,
		"total_out": u.TotalOut,
	})

	post.ViewerIDs = append(post.ViewerIDs, viewerID)
	post.ViewCount++
	viewerIDsJSON, _ := json.Marshal(post.ViewerIDs)
	db.Dao.Model(post).Updates(map[string]interface{}{
		"view_count": post.ViewCount,
		"viewer_ids": string(viewerIDsJSON),
	})

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
		db.Dao.Delete(&existing)
		db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("like_count", gorm.Expr("like_count - 1"))
		var post StrategyPost
		db.Dao.First(&post, postID)
		return false, post.LikeCount, nil
	}

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

	db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("comment_cnt", gorm.Expr("comment_cnt + 1"))

	addedPoints := int64(0)
	remain := int64(0)

	if userID != "" {
		var post StrategyPost
		db.Dao.First(&post, postID)

		if post.UserID != userID {
			hasImage := len(images) > 0
			textLen := len([]rune(content))
			if textLen > 10 || hasImage {
				today := time.Now().Format("2006-01-02")
				var todayReplyPoints int64
				db.Dao.Model(&StrategyPointsLog{}).
					Where("user_id = ? AND reason = 'reply' AND created_at >= ? AND created_at < ?",
						userID, today+" 00:00:00", today+" 23:59:59").
					Select("COALESCE(SUM(delta), 0)").
					Scan(&todayReplyPoints)

				if todayReplyPoints < 10 {
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

// GetComments 获取帖子评论
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

// DeleteComment 删除评论
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
	db.Dao.Model(&StrategyPost{}).Where("id = ?", postID).Update("comment_cnt", gorm.Expr("comment_cnt - 1"))

	var logs []StrategyPointsLog
	db.Dao.Where("ref_id = ? AND reason = ?", fmt.Sprintf("%d", comment.ID), "reply").Find(&logs)
	for _, log := range logs {
		if log.UserID == userID {
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

	db.Dao.Delete(&post)
	db.Dao.Where("post_id = ?", postID).Delete(&StrategyComment{})
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
