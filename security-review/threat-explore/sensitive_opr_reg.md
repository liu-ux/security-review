# 缺陷点分析任务

## 任务项

1、 识别单文件中可能存在缺陷的敏感操作
2、 区分敏感操作的可触发性&风险程度
3、 合并输出可触发的风险敏感操作项

## 前置条件

- 待审计代码文件（范围）

## 预定义漏洞类型 （仅检出以下类型的漏洞）

* `COMMAND_INJECTION`
* `SQL_INJECTION`
* `PATH_TRAVERSAL`
* `INSECURE_FILE_UPLOAD`
* `SSRF`
* `FORMAT_STRING_VULNERABILITY`
* `BUFFER_OVERFLOW`
* `DESERIALIZATION`
* `CODE_INJECTION`
* `JNDI_INJECTION`
* `UNSAFE_REFLECTION`
* `EXPRESSION_INJECTION`
* `SSTI`
* `LDAP_INJECTION`
* `ABUSE_OF_EMAIL_FUNCTIONALITY`
* `ABUSE_OF_SMS_FUNCTIONALITY`
* `AUTHENTICATION_BYPASS`
* `PRIVILEGE_ESCALATION`
* `UNAUTHORIZED_ACCESS`
* `SESSION_MANAGEMENT_FLAWS`
* `JWT_TOKEN_VULNERABILITY`
* `AUTHORIZATION_LOGIC_BYPASS`
* `INSECURE_DIRECT_OBJECT_REFERENCE`
* `HARD_CODED_CREDENIAL`
* `WEAK_ENCRYPTION_ALGORITHM`
* `USE_OF_INSECURE_RANDOM`
* `SENSITIVE_DATA_EXPOSURE`

## 分析步骤

! 严格按照下述步骤进行分析，不允许跳出流程或者裁剪步骤

### Step1 初步分析该代码涉及什么类型的敏感操作

#### Sink类敏感操作

- SQL 注入：数据库执行函数（如 cursor.execute、Session.execute、engine.execute）。
- 命令注入：系统调用函数（如 Python 的 os.system、subprocess.Popen，Java 的 Runtime.exec，Go 的 exec.Command）。
- 路径穿越：文件操作函数（如 open、os.open、os.path.join、java.io.FileInputStream、fs.readFile）。
- SSRF：服务端 HTTP 请求函数（如 Python 的 requests.get、urllib.request.urlopen，Java 的 HttpClient.send，Go 的 http.Get，Node.js 的 axios.get）。
- 反序列化：反序列化函数（如 Python 的 pickle.loads、yaml.load，Java 的 ObjectInputStream.readObject、XMLDecoder.readObject、fastjson.parseObject、fastjson.parse，PHP 的 unserialize）。
- 代码注入：代码执行函数（如 Python 的 eval、exec，Java 的 ScriptEngine.eval、GroovyShell.evaluate，PHP 的 eval、create_function）。
- JNDI注入：触发JNDI查找的函数（如 InitialContext.lookup、 InitialDirContext.search、 JndiTemplate.lookup等显式触发jdni查找的，还有如 JdbcRowSetImpl.connect、 JndiObjectFactoryBean.afterPropertiesSet等内部隐含jdni查找行为的）。
- 不安全反射：通过反射构造类或执行代码的函数（如 Class.newInstance、 Proxy.newProxyInstance、 Constructor.newInstance、 URLClassLoader.newInstance、 Method.invoke等)。
- 表达式注入：可执行表达式的函数（如 jakarta.el.ELProcessor.eval、ValueExpression.getValue 等，以及 JUEL、Tomcat 等 EL 实现中封装的执行 EL 表达式的函数）。
    - SpEL表达式：可执行SpEL表达式的函数（如 org.springframework.expression.Expression.getValue和其他spring系列框架封装的执行SpEL表达式的函数）。
    - ONGL表达式：可执行OGNL表达式的函数（如 TextParseUtil.translateVariables、 OgnlTextParser.evaluate、 Ognl.getValue等）。
- 模板注入： 模板渲染函数（如 org.thymeleaf.TemplateEngine.process、freemarker.template.Template.process、org.apache.velocity.Template.evaluate 等）。
- LDAP注入：可进行LDAP查询的函数（如 InitialLdapContext.search、InitialDirContext.search等）。
- 邮件功能滥用：邮件发送函数（如 org.springframework.mail.javamail.JavaMailSender.send、jakarta.mail.Transport.sendMessage、以及封装第三方邮件服务API的请求函数或者SDK等）
- 短信功能滥用：请求第三方短信服务的函数（如 各类短信SDK的短信发送函数， 封装第三方短信服务API的请求函数等）
- 认证绕过：涉及认证逻辑或者会话管理逻辑的函数 等
- 硬编码凭证：字面量作为敏感凭据的赋值代码、某函数调用安全校验相关函数使用字面量作为密钥/密码 等
- 格式化字符串漏洞：格式化输出函数（如 C/C++ 的 printf、sprintf、fprintf、snprintf，Python 的 % 格式化操作）。
- 缓冲区溢出：缓冲区溢出漏洞不要求必须调用特定标准库函数。只要代码中存在可能导致内存越界写入的语义行为，即应视为漏洞。如果存在内存操作函数（如 C/C++ 的 memcpy、strcpy、sprintf、wcscpy、wcscat、gets、strcat 等），则一定要视为缓冲区溢出漏洞。
- NoSQL注入：执行NoSQL查询的函数
- CSV注入：解析CSV内容的函数
- XXE注入：解析XML内容的函数

#### 业务逻辑类敏感操作

- 数据修改 (HIGH)
    - 识别特征：POST/PUT/DELETE
    - 典型示例：创建/修改/删除
- 数据访问 (MEDIUM)
    - 识别特征：GET + ID参数
    - 典型示例：/user/{id}  /用户/{id}
- 批量操作 (HIGH)
    - 识别特征：export/download/batch  导出/下载/批量
    - 典型示例：导出/批量删除
- 权限变更 (CRITICAL)
    - 识别特征：role/permission/grant  角色/权限/授予
    - 典型示例：角色/权限分配
- 资金操作 (CRITICAL)
    - 识别特征：transfer/pay/refund  转账/付款/退款
    - 典型示例：转账/支付/退款
- 认证操作 (CRITICAL)
    - 识别特征：login/password/token  登录/密码/令牌
    - 典型示例：登录/密码重置

#### 配置类缺陷

- 弱加密算法: 使用已弃用或弱加密算法（如 MD5、SHA1、DES 等）
- 伪随机数使用: 使用不安全的随机数生成器
- 敏感数据泄露: 敏感数据记录或存储、PII 处理违规、PII 处理违规、API 端点数据泄露、调试信息泄露等;合成密码、凭据、个人信息等打印日志里或者报错栈信息里出现

### Step2 识别存在风险的敏感操作列表

根据代码语言加载风险分析规则（必须）：

  - java: `<本SKILL根目录>/threat-explore/limits/java.md`
  - python: `<本SKILL根目录>/threat-explore/limits/python.md`
  - go: `<本SKILL根目录>/threat-explore/limits/go.md`
  - cpp: `<本SKILL根目录>/threat-explore/limits/cpp.md`
  - 其他：`<本SKILL根目录>/threat-explore/limits/default.md`

### Step3 分析敏感操作能否被触发

! 如果某个敏感操作无法触发或者没有风险，则无需检出
! 分析范围仅限于待审计文件路径，更大范围的上下文分析由其他任务完成，本任务不涉及

#### 外部输入列举

包括但不限于:

- 函数参数
- 请求体
- 环境变量
- 文件内容
- URL Query
- Header
- 表单
- ...

#### 分析输出规则

* 只识别 **预定义漏洞类型**，其他一律不报。
* 必须 **调用漏洞触发点** 才可报；否则即使校验不足也 **不报**。
* 对于注入类，若仅有 **可能风险/校验不足/未过滤** ，但 **没有触发点的实际调用** ，不报。
* 对于注入类，若 **外部输入** 是常量，不报。
* 对于注入类，存在 **外部输入** 影响关键参数，才认定为风险敏感操作
* 不得将整个函数体或仅函数定义行/函数签名行作为行号范围。

### Step4 输出敏感操作风险列表

输出识别到的敏感操作信息，包括以下维度：

- 代码文件路径+行号范围
- 关键操作代码片段
- 敏感操作描述+潜在危害

## 最佳实践

- 搜索代码
  - 错误做法: Grep搜索到敏感操作代码行后直接根据搜索到内容分析
  - 最佳实践: Grep到所在行后，读取所在行代码上下文，理清功能逻辑

