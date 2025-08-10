package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cocowh/muxcore/core/bus"
)

// 简化示例，不使用真实的WebSocket

func main() {
	// 创建增强消息总线
	config := &bus.BusConfig{
		MaxConnections:     100,
		MessageTimeout:     30 * time.Second,
		CleanupInterval:    5 * time.Minute,
		RetryAttempts:      3,
		BatchSize:          10,
		CompressionEnabled: true,
		EnableMetrics:      true,
		EnableTracing:      true,
		PartitionCount:     4,
		MaxQueueSize:       1000,
		EnablePersistence:  false,
	}

	// 创建内存消息存储
	messageStore := bus.NewInMemoryMessageStore()
	messageBus := bus.NewEnhancedMessageBus(config, messageStore)
	defer messageBus.Close()

	// 订阅事件
	messageBus.SubscribeEvent("connection.new", func(event *bus.Event) error {
		fmt.Printf("新连接事件: %+v\n", event)
		return nil
	})

	messageBus.SubscribeEvent("message.published", func(event *bus.Event) error {
		fmt.Printf("消息发布事件: %+v\n", event)
		return nil
	})

	// 模拟连接处理
	http.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		// 模拟连接ID
		connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

		// 订阅主题（模拟）
		messageBus.Subscribe(connID, "chat", func(ctx context.Context, msg *bus.Message) error {
			fmt.Printf("收到聊天消息: %s\n", msg.Payload)
			return nil
		}, nil)

		// 发布事件
		messageBus.PublishEvent(&bus.Event{
			Type: "connection.new",
			Data: map[string]interface{}{
				"connection_id": connID,
				"timestamp":     time.Now(),
			},
		})

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "演示连接 %s 已创建", connID)
	})

	// 定期发布测试消息
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 创建测试消息
				msg := &bus.Message{
					ID:           fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					Type:         bus.MessageTypeNotification,
					Topic:        "chat",
					Payload:      "定期测试消息",
					Timestamp:    time.Now(),
					Source:       "system",
					Priority:     bus.PriorityNormal,
					DeliveryMode: bus.DeliveryModeAtLeastOnce,
				}

				// 发布消息
				err := messageBus.Publish(context.Background(), msg)
				if err != nil {
					log.Printf("发布消息失败: %v", err)
				}

				// 打印指标
				metrics := messageBus.GetMetrics()
				fmt.Printf("总线指标 - 连接数: %d, 订阅数: %d, 已发布: %d, 已传递: %d\n",
					messageBus.GetConnectionCount(), messageBus.GetSubscriptionCount(),
					metrics.MessagesPublished, metrics.MessagesDelivered)
			}
		}
	}()

	// 启动HTTP服务器
	fmt.Println("消息总线示例服务器启动在 :8080")
	fmt.Println("演示端点: http://localhost:8080/demo")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
