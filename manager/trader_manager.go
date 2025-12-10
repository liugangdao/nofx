package manager

import (
	"fmt"
	"log"
	"nofx/config"
	"nofx/trader"
	"sync"
	"time"
)

// TraderManager 管理多个trader实例
type TraderManager struct {
	autoTraders      map[string]*trader.AutoTrader      // key: trader ID (mode=tm)
	positionManagers map[string]*trader.PositionManager // key: trader ID (mode=pm)
	mu               sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager() *TraderManager {
	return &TraderManager{
		autoTraders:      make(map[string]*trader.AutoTrader),
		positionManagers: make(map[string]*trader.PositionManager),
	}
}

// AddTrader 添加一个trader（根据mode创建AutoTrader或PositionManager）
func (tm *TraderManager) AddTrader(cfg config.TraderConfig, coinPoolURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, leverage config.LeverageConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 检查ID是否已存在
	if _, exists := tm.autoTraders[cfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", cfg.ID)
	}
	if _, exists := tm.positionManagers[cfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", cfg.ID)
	}

	// 根据模式创建不同的实例
	if cfg.Mode == "pm" {
		// 创建仓位管理器
		pmConfig := trader.PositionManagerConfig{
			ID:                    cfg.ID,
			Name:                  cfg.Name,
			AIModel:               cfg.AIModel,
			Exchange:              cfg.Exchange,
			EnableScreenshot:      cfg.EnableScreenshot,
			ScanInterval:          cfg.GetScanInterval(),
			ScanIntervalMinutes:   cfg.ScanIntervalMinutes,
			InitialBalance:        cfg.InitialBalance,
			BTCETHLeverage:        leverage.BTCETHLeverage,
			AltcoinLeverage:       leverage.AltcoinLeverage,
			BinanceAPIKey:         cfg.BinanceAPIKey,
			BinanceSecretKey:      cfg.BinanceSecretKey,
			HyperliquidPrivateKey: cfg.HyperliquidPrivateKey,
			HyperliquidWalletAddr: cfg.HyperliquidWalletAddr,
			HyperliquidTestnet:    cfg.HyperliquidTestnet,
			AsterUser:             cfg.AsterUser,
			AsterSigner:           cfg.AsterSigner,
			AsterPrivateKey:       cfg.AsterPrivateKey,
			DeepSeekKey:           cfg.DeepSeekKey,
			QwenKey:               cfg.QwenKey,
			GeminiKey:             cfg.GeminiKey,
			CustomAPIURL:          cfg.CustomAPIURL,
			CustomAPIKey:          cfg.CustomAPIKey,
			CustomModelName:       cfg.CustomModelName,
		}

		pm, err := trader.NewPositionManager(pmConfig)
		if err != nil {
			return fmt.Errorf("创建仓位管理器失败: %w", err)
		}

		tm.positionManagers[cfg.ID] = pm
		log.Printf("✓ 仓位管理器 '%s' (%s) 已添加", cfg.Name, cfg.AIModel)
	} else {
		// 创建交易机器人（默认模式）
		traderConfig := trader.AutoTraderConfig{
			ID:                    cfg.ID,
			Name:                  cfg.Name,
			AIModel:               cfg.AIModel,
			Exchange:              cfg.Exchange,
			BinanceAPIKey:         cfg.BinanceAPIKey,
			BinanceSecretKey:      cfg.BinanceSecretKey,
			HyperliquidPrivateKey: cfg.HyperliquidPrivateKey,
			HyperliquidWalletAddr: cfg.HyperliquidWalletAddr,
			HyperliquidTestnet:    cfg.HyperliquidTestnet,
			AsterUser:             cfg.AsterUser,
			AsterSigner:           cfg.AsterSigner,
			AsterPrivateKey:       cfg.AsterPrivateKey,
			CoinPoolAPIURL:        coinPoolURL,
			UseQwen:               cfg.AIModel == "qwen",
			DeepSeekKey:           cfg.DeepSeekKey,
			QwenKey:               cfg.QwenKey,
			GeminiKey:             cfg.GeminiKey,
			EnableScreenshot:      cfg.EnableScreenshot,
			CustomAPIURL:          cfg.CustomAPIURL,
			CustomAPIKey:          cfg.CustomAPIKey,
			CustomModelName:       cfg.CustomModelName,
			ScanInterval:          cfg.GetScanInterval(),
			ScanIntervalMinutes:   cfg.ScanIntervalMinutes,
			InitialBalance:        cfg.InitialBalance,
			BTCETHLeverage:        leverage.BTCETHLeverage,
			AltcoinLeverage:       leverage.AltcoinLeverage,
			MaxDailyLoss:          maxDailyLoss,
			MaxDrawdown:           maxDrawdown,
			StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		}

		at, err := trader.NewAutoTrader(traderConfig)
		if err != nil {
			return fmt.Errorf("创建交易机器人失败: %w", err)
		}

		tm.autoTraders[cfg.ID] = at
		log.Printf("✓ 交易机器人 '%s' (%s) 已添加", cfg.Name, cfg.AIModel)
	}

	return nil
}

// GetAutoTrader 获取指定ID的交易机器人
func (tm *TraderManager) GetAutoTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.autoTraders[id]
	if !exists {
		return nil, fmt.Errorf("交易机器人 ID '%s' 不存在", id)
	}
	return t, nil
}

// GetPositionManager 获取指定ID的仓位管理器
func (tm *TraderManager) GetPositionManager(id string) (*trader.PositionManager, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	pm, exists := tm.positionManagers[id]
	if !exists {
		return nil, fmt.Errorf("仓位管理器 ID '%s' 不存在", id)
	}
	return pm, nil
}

// GetTrader 获取指定ID的trader（兼容旧代码，优先返回AutoTrader）
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	return tm.GetAutoTrader(id)
}

// GetAllAutoTraders 获取所有交易机器人
func (tm *TraderManager) GetAllAutoTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.autoTraders {
		result[id] = t
	}
	return result
}

// GetAllPositionManagers 获取所有仓位管理器
func (tm *TraderManager) GetAllPositionManagers() map[string]*trader.PositionManager {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.PositionManager)
	for id, pm := range tm.positionManagers {
		result[id] = pm
	}
	return result
}

// GetAllTraders 获取所有交易机器人（兼容旧代码）
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	return tm.GetAllAutoTraders()
}

// GetTraderIDs 获取所有trader ID列表（包括交易机器人和仓位管理器）
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.autoTraders)+len(tm.positionManagers))
	for id := range tm.autoTraders {
		ids = append(ids, id)
	}
	for id := range tm.positionManagers {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 启动所有trader（包括交易机器人和仓位管理器）
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 启动所有Trader...")

	// 启动所有交易机器人
	for id, t := range tm.autoTraders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动交易机器人 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}

	// 启动所有仓位管理器
	for id, pm := range tm.positionManagers {
		go func(managerID string, posManager *trader.PositionManager) {
			log.Printf("▶️  启动仓位管理器 %s...", posManager.GetName())
			if err := posManager.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", posManager.GetName(), err)
			}
		}(id, pm)
	}
}

// StopAll 停止所有trader（包括交易机器人和仓位管理器）
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  停止所有Trader...")

	// 停止所有交易机器人
	for _, t := range tm.autoTraders {
		t.Stop()
	}

	// 停止所有仓位管理器
	for _, pm := range tm.positionManagers {
		pm.Stop()
	}
}

// GetComparisonData 获取对比数据（包括交易机器人和仓位管理器）
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.autoTraders)+len(tm.positionManagers))

	// 添加交易机器人数据
	for _, t := range tm.autoTraders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"trader_type":     "tm",
			"ai_model":        t.GetAIModel(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	// 添加仓位管理器数据
	for _, pm := range tm.positionManagers {
		status := pm.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":   pm.GetID(),
			"trader_name": pm.GetName(),
			"trader_type": "pm",
			"ai_model":    pm.GetAIModel(),
			"call_count":  status["call_count"],
			"is_running":  status["is_running"],
			"start_time":  status["start_time"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}
