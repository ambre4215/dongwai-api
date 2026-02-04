package cache

import (
	"dongwai_backend/internal/model"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// DictCache 线程安全的词典缓存
type DictCache struct {
	sync.RWMutex
	// 映射: 清洗后的单词 -> [ID列表]
	// 例如: "的" -> ["id_1", "id_2"]
	mapping map[string][]string
	maxLen  int
}

var GlobalDict *DictCache

// InitDictCache 初始化并从数据库加载全量词典
func InitDictCache(db *gorm.DB) error {
	GlobalDict = &DictCache{
		mapping: make(map[string][]string),
		maxLen:  0,
	}
	return GlobalDict.Reload(db)
}

// Reload 全量加载
func (c *DictCache) Reload(db *gorm.DB) error {
	c.Lock()
	defer c.Unlock()

	var vocabs []model.Vocab
	// 只查询需要的字段
	if err := db.Select("id, kanji").Find(&vocabs).Error; err != nil {
		return err
	}

	// 重置 map
	c.mapping = make(map[string][]string)
	c.maxLen = 0

	for _, v := range vocabs {
		c.addInternal(v.Kanji, v.ID)
	}
	return nil
}

// AddOrUpdate 动态添加/更新单个词（无需查库）
func (c *DictCache) AddOrUpdate(kanji, id string) {
	c.Lock()
	defer c.Unlock()
	c.addInternal(kanji, id)
}

// Remove 删除某个 ID 的引用
func (c *DictCache) Remove(kanji, id string) {
	c.Lock()
	defer c.Unlock()

	key := c.cleanKey(kanji)
	ids, exists := c.mapping[key]
	if !exists {
		return
	}

	// 过滤掉该 ID
	newIDs := make([]string, 0, len(ids))
	for _, existingID := range ids {
		if existingID != id {
			newIDs = append(newIDs, existingID)
		}
	}

	if len(newIDs) == 0 {
		delete(c.mapping, key)
	} else {
		c.mapping[key] = newIDs
	}
}

// Get 查找词
func (c *DictCache) Get(word string) ([]string, bool) {
	c.RLock()
	defer c.RUnlock()
	ids, ok := c.mapping[word]
	return ids, ok
}

// MaxLen 获取最大词长
func (c *DictCache) MaxLen() int {
	c.RLock()
	defer c.RUnlock()
	return c.maxLen
}

// addInternal 内部添加逻辑（不带锁）
func (c *DictCache) addInternal(kanji, id string) {
	key := c.cleanKey(kanji)

	// 检查 ID 是否已存在，防止重复
	exists := false
	for _, oldID := range c.mapping[key] {
		if oldID == id {
			exists = true
			break
		}
	}
	if !exists {
		c.mapping[key] = append(c.mapping[key], id)
	}

	rLen := len([]rune(key))
	if rLen > c.maxLen {
		c.maxLen = rLen
	}
}

// cleanKey 统一的清洗逻辑
func (c *DictCache) cleanKey(kanji string) string {
	k := strings.ReplaceAll(kanji, "~", "")
	k = strings.ReplaceAll(k, "～", "")
	return k
}
