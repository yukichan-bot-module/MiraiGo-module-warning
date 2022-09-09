package warning

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Logiase/MiraiGo-Template/bot"
	"github.com/Logiase/MiraiGo-Template/utils"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/MiraiGo/message"
	"github.com/yukichan-bot-module/MiraiGo-module-warning/internal/cache"
)

var instance *warning
var logger = utils.GetModuleLogger("com.aimerneige.warning")

type warning struct {
}

func init() {
	instance = &warning{}
	bot.RegisterModule(instance)
}

func (w *warning) MiraiGoModule() bot.ModuleInfo {
	return bot.ModuleInfo{
		ID:       "com.aimerneige.warning",
		Instance: instance,
	}
}

// Init 初始化过程
// 在此处可以进行 Module 的初始化配置
// 如配置读取
func (w *warning) Init() {
}

// PostInit 第二次初始化
// 再次过程中可以进行跨 Module 的动作
// 如通用数据库等等
func (w *warning) PostInit() {
}

// Serve 注册服务函数部分
func (w *warning) Serve(b *bot.Bot) {
	b.GroupMessageEvent.Subscribe(func(c *client.QQClient, msg *message.GroupMessage) {
		// 格式一定不对的返回
		if len(msg.Elements) < 2 {
			return
		}
		groupCode := msg.GroupCode
		// 一些变量
		isAt := false
		isWarning := false
		isClean := false
		var target int64
		// 解析指令
		for _, ele := range msg.Elements {
			switch e := ele.(type) {
			case *message.AtElement:
				if !isAt {
					isAt = true
					target = e.Target
				}
			case *message.TextElement:
				contentStr := e.Content
				contentStr = strings.TrimSpace(contentStr)
				if !isWarning {
					if contentStr == "警告" {
						isWarning = true
					}
				}
				if !isClean {
					if contentStr == "清除警告" {
						isClean = true
					}
				}
			}
		}
		// 格式不正确，退出
		if !isAt || target == 0 {
			return
		}
		// 都不是，退出
		if !isWarning && !isClean {
			return
		}
		// 如果用户在点炒饭
		if isWarning && isClean {
			c.SendGroupMessage(groupCode, simpleText("不许点炒饭！"))
			return
		}
		// 检查 bot 管理员权限
		botMemberInfo, err := c.GetMemberInfo(groupCode, c.Uin)
		if err != nil {
			errMsg := fmt.Sprintf("在群「%d」获取成员「%d」的用户数据时发成错误，详情请查阅后台日志。", groupCode, msg.Sender.Uin)
			logger.WithError(err).Error(errMsg)
			c.SendGroupMessage(groupCode, simpleText(errMsg))
			return
		}
		if botMemberInfo.Permission != client.Administrator && botMemberInfo.Permission != client.Owner {
			c.SendGroupMessage(groupCode, simpleText("请先授予机器人管理员权限。"))
			return
		}
		// 检查发送者管理员权限
		senderMemberInfo, err := c.GetMemberInfo(groupCode, msg.Sender.Uin)
		if err != nil {
			errMsg := fmt.Sprintf("在群「%d」获取成员「%d」的用户数据时发成错误，详情请查阅后台日志。", groupCode, msg.Sender.Uin)
			logger.WithError(err).Error(errMsg)
			c.SendGroupMessage(groupCode, simpleText(errMsg))
			return
		}
		if senderMemberInfo.Permission != client.Administrator && senderMemberInfo.Permission != client.Owner {
			c.SendGroupMessage(groupCode, simpleText("乱玩指令会被管理员警告呢~"))
			return
		}
		// 警告
		if isWarning {
			// 如果警告对象是机器人
			if target == c.Uin {
				c.SendGroupMessage(groupCode, simpleText("不要警告我啊~"))
				return
			}
			// 检查目标用户权限
			targetMemberInfo, err := c.GetMemberInfo(groupCode, target)
			if err != nil {
				errMsg := fmt.Sprintf("在群「%d」获取成员「%d」的用户数据时发成错误，详情请查阅后台日志。", groupCode, target)
				logger.WithError(err).Error(errMsg)
				c.SendGroupMessage(groupCode, simpleText(errMsg))
				return
			}
			// 如果是群主
			if targetMemberInfo.Permission == client.Owner {
				c.SendGroupMessage(groupCode, simpleText("你居然想警告群主？真是危险的想法呢~"))
				return
			}
			// 如果是管理员
			if targetMemberInfo.Permission == client.Administrator {
				c.SendGroupMessage(groupCode, simpleText("警告管理员没有用的~"))
				return
			}
			// 警告
			if err = increaseWarningRecord(groupCode, target); err != nil {
				errMsg := fmt.Sprintf("在群「%d」尝试记录成员「%d」的警告次数时发送错误，详情请查阅后台日志。", groupCode, target)
				logger.WithError(err).Error(errMsg)
				c.SendGroupMessage(groupCode, simpleText(errMsg))
				return
			}
			// 检查被警告次数
			times, err := checkWarningRecord(groupCode, target)
			if err != nil {
				errMsg := fmt.Sprintf("在群「%d」尝试检查成员「%d」的被警告次数时发生错误，详情请查阅后台日志。", groupCode, target)
				logger.WithError(err).Error(errMsg)
				c.SendGroupMessage(groupCode, simpleText(errMsg))
				return
			}
			// 超过三次
			if times >= 3 {
				replyMsg := fmt.Sprintf("你已被管理员警告 %d 次，按照规定将被移出群。", times)
				c.SendGroupMessage(groupCode, withAt(target, replyMsg))
				// 踢出
				if err := targetMemberInfo.Kick("多次警告", false); err != nil {
					errMsg := fmt.Sprintf("在将成员「%d」移出群「%d」的过程中发生错误，详情请查阅后台日志。", target, groupCode)
					logger.WithError(err).Error(errMsg)
					c.SendGroupMessage(groupCode, simpleText(errMsg))
					return
				}
				return
			}
			// 未超过三次
			replyMsg := fmt.Sprintf("你已被管理员警告 %d 次，如果累计超过三次将会被移出本群！", times)
			c.SendGroupMessage(groupCode, withAt(target, replyMsg))
			return
		}
		// 清除
		if isClean {
			if err := cleanWarningRecord(groupCode, target); err != nil {
				replyMsg := fmt.Sprintf("在群「%d」尝试清空群成员「%d」的警告次数时发生错误，详情请查阅后台日志。", groupCode, target)
				logger.WithError(err).Error(replyMsg)
				c.SendGroupMessage(groupCode, simpleText(replyMsg))
				return
			}
			replyMsg := fmt.Sprintf("清除用户「%d」警告次数成功！", target)
			c.SendGroupMessage(groupCode, simpleText(replyMsg))
		}
	})
}

// Start 此函数会新开携程进行调用
// ```go
//
//	go exampleModule.Start()
//
// ```
// 可以利用此部分进行后台操作
// 如 http 服务器等等
func (w *warning) Start(b *bot.Bot) {
}

// Stop 结束部分
// 一般调用此函数时，程序接收到 os.Interrupt 信号
// 即将退出
// 在此处应该释放相应的资源或者对状态进行保存
func (w *warning) Stop(b *bot.Bot, wg *sync.WaitGroup) {
	// 别忘了解锁
	defer wg.Done()
}

func increaseWarningRecord(groupCode int64, memberUin int64) error {
	currentTimes, err := checkWarningRecord(groupCode, memberUin)
	if err != nil {
		return err
	}
	currentTimes++
	key := getRedisKey(groupCode, memberUin)
	return cache.SetCache(key, fmt.Sprint(currentTimes))
}

func cleanWarningRecord(groupCode int64, memberUin int64) error {
	key := getRedisKey(groupCode, memberUin)
	return cache.SetCache(key, "0")
}

func checkWarningRecord(groupCode int64, memberUin int64) (int, error) {
	key := getRedisKey(groupCode, memberUin)
	dataString, err := cache.GetKeyOrSetCache(key, func() (string, error) {
		return fmt.Sprintf("0"), nil
	})
	if err != nil {
		return -1, err
	}
	timesI64, err := strconv.ParseInt(dataString, 10, 32)
	return int(timesI64), err
}

func getRedisKey(groupCode, senderUin int64) string {
	return fmt.Sprintf("bot-com-aimerneige-warning-%d-%d", groupCode, senderUin)
}

func withAt(target int64, s string) *message.SendingMessage {
	return message.NewSendingMessage().Append(message.NewAt(target)).Append(message.NewText(s))
}

func simpleText(s string) *message.SendingMessage {
	return message.NewSendingMessage().Append(message.NewText(s))
}
