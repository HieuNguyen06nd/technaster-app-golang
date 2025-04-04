# Ứng dụng Học Ngoại Ngữ - Backend Service
## Giới thiệu dự án

Dự án xây dựng hệ thống backend phục vụ ứng dụng học đa ngôn ngữ, tích hợp các dịch vụ AI để:
- Tạo và xử lý nội dung học tập
- So sánh và phân tích dữ liệu
- Chuyển đổi văn bản thành giọng nói (SSML)
- Tạo hội thoại và trích xuất từ vựng

## Công nghệ sử dụng
- **Framework**: Iris (Golang)
- **Cơ sở dữ liệu**: PostgreSQL
- **AI Services**: Groq API, DeepSeek, Quwen
- **Frontend**: HTML5, JavaScript, CSS3

## Cài đặt

### Yêu cầu hệ thống
- Go 1.18+
- PostgreSQL 14+
### Bước 1: Cài đặt database
	createdb language_learning

### Bước 2: Cấu hình biến môi trường( đối với 03)
  const (
	host     = "localhost" //Địa chỉ máy chủ cơ sở dữ liệu.
	port     = 5432 //Cổng kết nối đến PostgreSQL
	user     = "postgres" //Tên người dùng để đăng nhập vào PostgreSQL.
	password = "123456" //Mật khẩu của người dùng. 
	dbname   = "language_db" // Tên cơ sở dữ liệu muốn kết nối. Ở đây là "language_db"
	groqAPI  = "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" // Khóa API để xác thực với dịch vụ Groq
)
Thay đổi tùy thuộc vào local trên máy

### Bước 3: Chạy ứng dụng
  # Khởi chạy server
  go run main.go

Ảnh Minh họa: 

### 01
![image](https://github.com/user-attachments/assets/54110254-0af0-40dc-ba72-1827f6398a18)
### 02
![image](https://github.com/user-attachments/assets/37a0752e-e288-4c6e-8251-d110c81353bd)
### 03
![image](https://github.com/user-attachments/assets/0508902f-a5c5-42ae-b056-5cfeeb16cc00)

