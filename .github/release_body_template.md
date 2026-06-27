## FileCodeBox-Go ${{ github.ref_name }}

匿名口令分享文本和文件

### Docker 部署
```bash
docker pull ghcr.io/${{ github.repository }}:${{ github.ref_name }}
docker run -d -p 12345:12345 -v ./data:/app/data ghcr.io/${{ github.repository }}:${{ github.ref_name }}
```

### 二进制文件
见下方 Assets，下载对应平台的可执行文件直接运行。
