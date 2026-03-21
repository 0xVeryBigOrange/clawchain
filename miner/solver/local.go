package solver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// solveMathLocal attempts to solve math challenges without LLM.
// Handles: "计算 X op Y 的结果" where op is +, -, *
func solveMathLocal(prompt string) (string, bool) {
	re := regexp.MustCompile(`计算\s*(\d+)\s*([+\-*×])\s*(\d+)`)
	m := re.FindStringSubmatch(prompt)
	if m == nil {
		return "", false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	op := m[2]
	var result int
	switch op {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*", "×":
		result = a * b
	default:
		return "", false
	}
	return fmt.Sprintf("%d", result), true
}

// solveLogicLocal attempts to solve logic challenges without LLM.
func solveLogicLocal(prompt string) (string, bool) {
	p := strings.ToLower(prompt)
	if strings.Contains(p, "a > b") && strings.Contains(p, "b > c") {
		return "A>C", true
	}
	if strings.Contains(p, "a < b") && strings.Contains(p, "b < c") {
		return "A<C", true
	}
	return "", false
}

// solveClassificationLocal classifies text into 5 categories.
// Categories: 科技/金融/体育/娱乐/政治
func solveClassificationLocal(prompt string) (string, bool) {
	p := strings.ToLower(prompt)

	techKeywords := []string{"ai", "gpt", "openai", "spacex", "blockchain", "算法", "芯片", "量子", "模型", "科技", "技术", "gpu", "编程", "软件", "硬件", "5g", "cloud", "robot", "自动驾驶", "人工智能"}
	financeKeywords := []string{"股", "基金", "利率", "美联储", "央行", "通胀", "gdp", "比特币", "加密", "金融", "投资", "市场", "经济", "银行", "贸易", "汇率", "债券"}
	sportsKeywords := []string{"世界杯", "奥运", "冠军", "联赛", "球", "运动员", "体育", "nba", "足球", "篮球", "网球", "赛事", "决赛", "梅西"}
	entertainmentKeywords := []string{"电影", "票房", "演员", "导演", "综艺", "歌手", "音乐", "娱乐", "明星", "剧", "节目", "奖项"}
	politicsKeywords := []string{"联合国", "国会", "总统", "政府", "政策", "选举", "外交", "峰会", "制裁", "安理会", "政治", "议会", "国防", "军事"}

	score := map[string]int{"科技": 0, "金融": 0, "体育": 0, "娱乐": 0, "政治": 0}
	for _, kw := range techKeywords {
		if strings.Contains(p, kw) { score["科技"]++ }
	}
	for _, kw := range financeKeywords {
		if strings.Contains(p, kw) { score["金融"]++ }
	}
	for _, kw := range sportsKeywords {
		if strings.Contains(p, kw) { score["体育"]++ }
	}
	for _, kw := range entertainmentKeywords {
		if strings.Contains(p, kw) { score["娱乐"]++ }
	}
	for _, kw := range politicsKeywords {
		if strings.Contains(p, kw) { score["政治"]++ }
	}

	best := "科技" // default
	bestScore := 0
	for cat, s := range score {
		if s > bestScore {
			bestScore = s
			best = cat
		}
	}
	if bestScore == 0 {
		return "", false
	}
	return best, true
}

// solveSentimentLocal classifies sentiment as positive/negative/neutral.
func solveSentimentLocal(prompt string) (string, bool) {
	p := strings.ToLower(prompt)

	posWords := []string{"好", "棒", "优秀", "成功", "增长", "突破", "喜", "赞", "创新", "beautiful", "great", "good", "excellent", "happy"}
	negWords := []string{"差", "坏", "失败", "下降", "危机", "问题", "担忧", "悲", "terrible", "bad", "poor", "crisis", "sad", "fail"}

	pos, neg := 0, 0
	for _, w := range posWords {
		if strings.Contains(p, w) { pos++ }
	}
	for _, w := range negWords {
		if strings.Contains(p, w) { neg++ }
	}

	if pos > neg { return "positive", true }
	if neg > pos { return "negative", true }
	if pos == 0 && neg == 0 { return "", false }
	return "neutral", true
}
