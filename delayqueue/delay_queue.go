package delayqueue

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/php-cpm/delay-queue/config"
)

var (
	// 每个定时器对应一个bucket
	timers []*time.Ticker
	// bucket名称chan
	bucketNameChan <-chan string
)

// Init 初始化延时队列
func Init() {
	// 初始化 redis 连接池
	RedisPool = initRedisPool()

	// 初始化一些列 Timer
	initTimers()

	// golang 函数：http://blog.csdn.net/mungo/article/details/52481285
	// 进行初始化，产生一个 go routine
	bucketNameChan = generateBucketName()
}

// Push 添加一个Job到队列中
func Push(job Job) error {
	if job.Id == "" || job.Topic == "" || job.Delay < 0 || job.TTR <= 0 {
		return errors.New("invalid job")
	}

	err := putJob(job.Id, job)
	if err != nil {
		log.Printf("添加job到job pool失败#job-%+v#%s", job, err.Error())
		return err
	}

	// 轮询的方式存放。ZADD 命令 。存放在有序集合中。将会从这里获取 要运行的 job
	err = pushToBucket(<-bucketNameChan, job.Delay, job.Id)
	if err != nil {
		log.Printf("添加job到bucket失败#job-%+v#%s", job, err.Error())
		return err
	}

	return nil
}

// Pop 轮询获取Job
func Pop(topics []string) (*Job, error) {
	// ready queue 里面只有 topic  作为 key
	// jobId, err := blockPopFromReadyQueue(topics, config.Setting.QueueBlockTimeout)
	jobId, err := blPopFromReadyQueue(topics, config.Setting.QueueBlockTimeout)
	if err != nil {
		return nil, err
	}

	// 队列为空
	if jobId == "" {
		return nil, nil
	}

	// 获取job元信息
	job, err := getJob(jobId)
	if err != nil {
		return job, err
	}

	// 消息不存在, 可能已被删除
	if job == nil {
		return nil, nil
	}

	// 重新放到 Bucket 中，等待重新消费。实现至少一次的逻辑。如果客户端删除了 job ，那么。调度到此 jobId 的时候，发现 job 不存在，直接在 bucket 中删除
	timestamp := time.Now().Unix() + job.TTR
	// 表示从 <-bucketNameChan 。这个 channel 接收一个值
	err = pushToBucket(<-bucketNameChan, timestamp, job.Id) //待确认的消息放入bucket1

	return job, err
}

// Remove 删除Job
func Remove(jobId string) error {
	return removeJob(jobId)
}

// Get 查询Job
func Get(jobId string) (*Job, error) {
	job, err := getJob(jobId)
	if err != nil {
		return job, err
	}

	// 消息不存在, 可能已被删除
	if job == nil {
		return nil, nil
	}
	return job, err
}

// 轮询获取bucket名称, 使job分布到不同bucket中, 提高扫描速度
func generateBucketName() <-chan string {
	// 阻塞 channel
	c := make(chan string)
	// 1、为什么这么写呢？为什么不直接写个 for 死循环呢？
	// 如果直接写 for 循环，那在初始化的时候，会阻塞其他 init 函数。如果另起一个 go routine 的话，就不会阻塞其他的

	// 2、每次都 产生一个 go routine 。是怎么销毁的呀？
	// 因为把这个函数 赋给某个 变量了。在 init 中初始化了。只有一个 go routine 。

	// 3、我感觉到 里面的 i 变量好像没有作用的呀，因为都没有和其他 go routine 交换。
	// 因为在 初始化 init 一下。每次都从 bucketNameChan 这个 channel 读取信息。
	go func() {
		i := 1

		// 死循环
		for {
			// chan <-  发送消息
			c <- fmt.Sprintf(config.Setting.BucketName, i)
			if i >= config.Setting.BucketSize {
				i = 1
			} else {
				i++
			}
		}
	}()

	return c
}

// 初始化定时器 https://yq.aliyun.com/articles/69303
func initTimers() {
	timers = make([]*time.Ticker, config.Setting.BucketSize)
	var bucketName string
	for i := 0; i < config.Setting.BucketSize; i++ {

		// 每 1s 执行一次
		timers[i] = time.NewTicker(1 * time.Second)

		// 如果这里部署多实例的话，就会产生竞争
		bucketName = fmt.Sprintf(config.Setting.BucketName, i+1)

		// 并发执行
		go waitTicker(timers[i], bucketName)
	}
}

func waitTicker(timer *time.Ticker, bucketName string) {
	for {
		select {
		case t := <-timer.C: //  我们启动一个新的goroutine，来以阻塞的方式从Timer的C这个channel中，等待接收一个值，这个值是到期的时间。
			tickHandler(t, bucketName)
		}
	}
}

// 扫描bucket, 取出延迟时间小于当前时间的Job
func tickHandler(t time.Time, bucketName string) {
	for {
		// 拿到第一个元素。bucket 存放 jobid 和时间戳
		bucketItem, err := getFromBucket(bucketName)
		if err != nil {
			log.Printf("扫描bucket错误#bucket-%s#%s", bucketName, err.Error())
			return
		}

		// 集合为空
		if bucketItem == nil {
			return
		}

		// 延迟时间未到
		if bucketItem.timestamp > t.Unix() {
			return
		}

		// 延迟时间小于等于当前时间, 取出Job元信息并放入ready queue
		job, err := getJob(bucketItem.jobId)
		if err != nil {
			log.Printf("获取Job元信息失败#bucket-%s#%s", bucketName, err.Error())
			continue
		}

		// job元信息不存在, 从bucket中删除
		if job == nil {
			removeFromBucket(bucketName, bucketItem.jobId)
			continue
		}

		// 再次确认元信息中delay是否小于等于当前时间
		if job.Delay > t.Unix() {
			// 从bucket中删除旧的jobId
			removeFromBucket(bucketName, bucketItem.jobId)
			// 重新计算delay时间并放入其他的 bucket 中
			pushToBucket(<-bucketNameChan, job.Delay, bucketItem.jobId)
			continue
		}

		// 放到 Ready 队列中，普通的 redis list 即可。RPUSH 方式
		err = pushToReadyQueue(job.Topic, bucketItem.jobId)
		if err != nil {
			log.Printf("JobId放入ready queue失败#bucket-%s#job-%+v#%s",
				bucketName, job, err.Error())
			continue
		}

		// 从bucket中删除
		removeFromBucket(bucketName, bucketItem.jobId)
	}
}
