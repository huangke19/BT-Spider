# BT-Spider 🕷

个人 BT 下载工具，粘贴磁力链接自动下载。

## 使用

```bash
# 编译
go build -o bt-spider .

# 运行（默认下载到 ~/Downloads/BT-Spider/）
./bt-spider

# 指定下载目录
./bt-spider /path/to/download

# 然后粘贴磁力链接即可
magnet> magnet:?xt=urn:btih:xxxxx...
```

## 功能

- [x] 磁力链接解析下载
- [x] 实时进度显示
- [x] 优雅退出（Ctrl+C）
- [ ] 种子文件支持
- [ ] 下载限速
- [ ] Web UI
