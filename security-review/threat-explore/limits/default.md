## 注入类漏洞限制

**触发点** 必须为 **官方标准库函数或常见第三方库函数**，并与漏洞类型 **一一对应**。

### 漏洞类型限制（只能识别以下类型）
* `COMMAND_INJECTION`
* `SQL_INJECTION`
* `PATH_TRAVERSAL`
* `INSECURE_FILE_UPLOAD`
* `SSRF`
* `DESERIALIZATION`
* `CODE_INJECTION`
* `ABUSE_OF_EMAIL_FUNCTIONALITY`
* `ABUSE_OF_SMS_FUNCTIONALITY`

## 业务逻辑类漏洞限制

### 预定义漏洞类型（只能识别以下类型）

* `AUTHENTICATION_BYPASS`
* `PRIVILEGE_ESCALATION`
* `UNAUTHORIZED_ACCESS`
* `SESSION_MANAGEMENT_FLAWS`
* `JWT_TOKEN_VULNERABILITY`
* `AUTHORIZATION_LOGIC_BYPASS`
* `INSECURE_DIRECT_OBJECT_REFERENCE`
* `HARD_CODED_CREDENIAL`

## 配置类漏洞限制

### 预定义漏洞类型（只能识别以下类型）

* `WEAK_ENCRYPTION_ALGORITHM`
* `USE_OF_INSECURE_RANDOM`
* `SENSITIVE_DATA_EXPOSURE`

## 安全知识
### 规则与边界
* 对 COMMAND_INJECTION
  * 拼接/执行命令。
* 对 SQL_INJECTION
  * 仅关注字符串拼接部分。
* 对 PATH_TRAVERSAL
  * 触发点必须为“实际文件操作”（读/写/创建/删除）。
* 对 SSRF
  * 需要可控参数发起HTTP请求。
* 对 DESERIALIZATION
  * 触发点必须为执行"反序列化操作"的函数（如 pickle.loads、ObjectInputStream.readObject、unserialize等）。
  * 反序列化的数据源来自未信任输入且未被充分校验。
* 对 COMMAND_INJECTION
  * 拼接/执行代码
* 对 ABUSE_OF_EMAIL_FUNCTIONALITY
  * 无需外部输入因素，邮件发送功能只要存在就有可能被滥用
* 对 ABUSE_OF_SMS_FUNCTIONALITY
  * 无需外部输入因素，SMS短信发送功能只要存在就有可能被滥用
* 对于 AUTHENTICATION_BYPASS
  * 认证逻辑存在缺陷的，如 参数校验可被绕过、重置密码的链接可被预测、依赖不可信字段做校验（如Refer报头）、缺失校验（如 校验用户名不校验密码、空密码放行、固定密码、超级session）、会话凭证生成缺乏随机性可被预测（如 基于用户ID生成） 等可导致认证逻辑被绕过
* 对于 HARD_CODED_CREDENIAL
  * 仅检测认证相关的凭据，范围包括但不限于 密钥、密码、Token 等
  * 全局作用域中的赋值代码也检出

### 反例（不报）
* 仅有参数合法性校验不足，仅打印/记录用户输入，未调用漏洞触发点，不报。
* 对 SQL_INJECTION
  * NoSQL（Mongo 等）相关调用，不报。
  * 未直接拼接SQL语句，使用参数化传递，或转义污点参数，防御SQL注入，不报。
* 对 PATH_TRAVERSAL
  * 仅判断文件是否存在，未做读/写/删除/创建操作，不报。
  * 仅发起 HTTP 请求（如 proxy.ServeHTTP 等）且与文件操作无关，不报。
  * 仅调用 redis.Client.Del 等缓存/数据库删除函数，不涉及文件系统，不报。
  * 仅调用 stat/lstat/fstat 等获取元数据的函数，且未见随后的写/创建/删除/重命名等调用链，不报。
  * 仅控制文件内容，无法控制文件路径，不报
  * 污点参数过滤目录穿越字符，不报。
  * 仅为日志目录初始化（路径非受控输入），不报。
  * 不可确认为系统/常见库的第三方函数，且无法证明其会做文件读写操作，不报。
* 对 SSRF
  * 不调用 标准库/第三方库 发起请求，不报
  * 调用自定义函数发起请求，不报
  * URL 不可控的不报。
  * 污点参数有 URL/IP 白名单校验，无法随意发起请求，不报
* 对 PATH_TRAVERSAL 和 INSECURE_FILE_UPLOAD
  * 对象存储Amazon s3、oss等无法目录穿越，不报
* 对 DESERIALIZATION
  * 使用安全的反序列化方法（如 Python 的 json.loads、Java 的 Gson.fromJson、.NET 的 DataContractJsonSerializer.ReadObject），不报。
  * 不可确认为系统/常见库的第三方反序列化函数，且无法证明其会执行任意代码，不报。
* 对 ABUSE_OF_SMS_FUNCTIONALITY
  * 对于请求API接口的函数，如果没有明显代码显示该接口涉及SMS服务请求，不报
* 对 ABUSE_OF_EMAIL_FUNCTIONALITY
  * 对于请求API接口的函数，如果没有明显代码显示该接口涉及邮件发送请求，不报
