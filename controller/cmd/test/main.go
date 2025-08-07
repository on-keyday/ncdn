package main

import (
	"context"

	"fmt"

	"sync"

	"time"
)

// あなたのコードのWaitGroupロジックを完全に再現

type YourMessageChannel struct {
	wg sync.WaitGroup

	ch chan int

	ctx context.Context

	cancel     context.CancelFunc
	cancelLock sync.RWMutex

	closeOnce sync.Once
}

func NewYourMessageChannel() *YourMessageChannel {

	ctx, cancel := context.WithCancel(context.Background())

	return &YourMessageChannel{

		ch: make(chan int, 100),

		ctx: ctx,

		cancel: cancel,
	}

}

// あなたのSendMessageロジック

func (c *YourMessageChannel) SendMessage(i int) {

	// このゴルーチンは、panicをキャッチして報告する役割

	defer func() {

		if r := recover(); r != nil {

			fmt.Printf("\n--- PANICを検知！: %v ---\n", r)

		}

	}()
	c.cancelLock.RLock()
	select {

	case <-c.ctx.Done():
		c.cancelLock.RUnlock()
		// fmt.Println(">>> ctx.Done()が呼び出されました。メッセージを送信しません。")

		return

	default:

	}

	c.wg.Add(1)
	c.cancelLock.RUnlock()

	go func() {

		defer c.wg.Done()

		c.ch <- i // ここでpanicが起きる可能性がある
		fmt.Printf("%d:>>> メッセージを送信しました。\n", i)

	}()

}

// あなたのCloseChannelロジック

func (c *YourMessageChannel) CloseChannel() {

	c.closeOnce.Do(func() {

		fmt.Println("\n>>> CloseChannel開始！ cancelを呼び出し...")
		c.cancelLock.Lock()
		c.cancel()
		c.cancelLock.Unlock()

		fmt.Println(">>> Waitで待機開始...")

		c.wg.Wait()

		fmt.Println(">>> Waitが終了！チャネルを閉鎖します。")

		close(c.ch)

		for range c.ch {

		}

	})

}

func (c *YourMessageChannel) GetChannel() <-chan int {

	return c.ch

}

func main() {

	channel := NewYourMessageChannel()

	// 0.1秒後にシャットダウン処理を開始する

	time.AfterFunc(100*time.Millisecond, func() {
		channel.CloseChannel()

	})

	go func() {

		for i := range channel.GetChannel() {

			fmt.Printf("%d:<<< メッセージを受信しました。\n", i)

		}

	}()

	// シャットダウン処理と競争するように、大量のメッセージを送り続ける

	for i := 0; i < 50000; i++ {

		go channel.SendMessage(i)

		// わずかな遅延がレースコンディションを誘発しやすくする

		time.Sleep(10 * time.Microsecond)

	}

	fmt.Println("メッセージ送信を開始しました。10秒後にテストを終了します。")

	// 全体が終了するのを待つ

	time.Sleep(10 * time.Second)

	fmt.Println("\nテスト終了。")

}
