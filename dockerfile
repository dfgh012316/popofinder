FROM python:3.10-slim

# 建立非 root 使用者，並設定 UID 為 1000 (與 Helm Chart 預設值一致)
RUN useradd -m -u 1000 appuser

WORKDIR /app

# 先安裝依賴，利用 Docker Layer Cache
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 複製程式碼
COPY . .

# 修正目錄權限，確保 appuser 可以讀取
RUN chown -R appuser:appuser /app

# 切換到非 root 使用者
USER appuser

# 設定 Python 環境變數
# PYTHONDONTWRITEBYTECODE: 避免產生 .pyc 檔案，有利於唯讀檔案系統
# PYTHONUNBUFFERED: 讓日誌能即時輸出
ENV PYTHONPATH=/app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

EXPOSE 8000

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
