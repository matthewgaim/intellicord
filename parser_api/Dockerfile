# Use the official Python image from the Docker Hub
FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

EXPOSE 8081

CMD ["gunicorn", "-k", "gevent", "--bind", "0.0.0.0:8081", "app:app"]
