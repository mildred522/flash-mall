package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// 配置
const (
	ConcurrentNum = 50                                       // 并发协程数 (模拟50个线程同时发请求)
	TotalRequests = 2000                                     // 总共发多少个请求
	ApiUrl        = "http://localhost:8888/api/order/create" // 下单接口
)

func main() {
	var successCount int64
	var failCount int64

	// 任务通道，控制总请求数
	jobs := make(chan int, TotalRequests)
	// 填满任务
	for i := 1; i <= TotalRequests; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(ConcurrentNum)

	fmt.Printf("🔥 开始压测：模拟 %d 并发，共 %d 次抢购...\n", ConcurrentNum, TotalRequests)
	startTime := time.Now()

	// 启动消费者协程
	for i := 0; i < ConcurrentNum; i++ {
		go func() {
			defer wg.Done()
			for uid := range jobs {
				// 模拟不同用户 ID (避免被可能的防抖逻辑拦截)
				// 每个人买 1 个
				jsonStr := fmt.Sprintf(`{"user_id": %d, "product_id": 1, "amount": 1}`, uid+10000)

				resp, err := http.Post(ApiUrl, "application/json", bytes.NewBuffer([]byte(jsonStr)))
				if err != nil {
					// 网络错误
					atomic.AddInt64(&failCount, 1)
					continue
				}

				// 200 表示下单成功
				if resp.StatusCode == 200 {
					atomic.AddInt64(&successCount, 1)
				} else {
					// 可能是库存不足(虽然后端有Redis拦截，但HTTP层可能返回其他码)
					atomic.AddInt64(&failCount, 1)
				}
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)
	qps := float64(TotalRequests) / duration.Seconds()

	fmt.Println("\n=========== 🏁 压测报告 ===========")
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("成功单数: %d\n", successCount)
	fmt.Printf("失败单数: %d\n", failCount)
	fmt.Printf("QPS (TPS): %.2f\n", qps)
	fmt.Println("===================================")
}
