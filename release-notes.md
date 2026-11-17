### 新特性与优化 (Features & Improvements - v0.1.36)

- **YouTube Music 在线智能匹配网易云歌词与双语翻译 (NetEase Lyrics Match for YouTube Music)**:
  - **网易云精准时间轴与翻译匹配**：播放 YouTube Music 插件歌曲时，自动优先匹配网易云音乐对应歌曲的高质量逐句时间轴歌词（LRC）与中文翻译，在客户端「正在播放」界面与「桌面歌词」中完美呈现双语滚动。
  - **YouTube 杂质标题智能净化**：针对 YouTube 常见的视频命名习惯，自动剔除标题中的修饰后缀（如 `(Official Music Video)`、`[Official MV]`、`【MV】`、`(Official Audio)`、`(Lyric Video)`、`[Live]` 等），并自动剔除艺术家名中的 `- Topic` 后缀，大幅提升匹配命中率。
  - **歌手/歌名智能解构**：针对包含 `歌手 - 歌名` 格式的视频标题，自动解析提取独立艺术家与歌曲名称分别检索。
  - **候选曲目深度甄别**：多候选搜索结果中自动优先选取包含逐行歌词的完整版本，杜绝纯纯伴奏/翻唱空白歌词。
  - **无缝回退机制**：当网易云未收录该歌曲时，自动无缝降级采用 YouTube Music 自带的原生歌词；本地音乐及其他插件源继续保持歌词智能补全。
  - **高清封面智能补全**：当 YouTube 音频封面缺失或低清时，自动匹配同步网易云高清专辑封面。

### 包含文件 (Files)
- `mygo-music-setup-0.1.36-amd64.exe`: Windows 官方风格安装程序 (NSIS Installer)
- `MyGo.Music.exe`: Windows 64位独立便携版 (绿色免安装)
