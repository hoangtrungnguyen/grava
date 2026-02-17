
-- Create a user specifically for external clients like DataGrip
-- Use mysql_native_password for maximum compatibility
CREATE USER IF NOT EXISTS 'grava_client'@'%' IDENTIFIED WITH mysql_native_password BY 'password';
GRANT ALL PRIVILEGES ON *.* TO 'grava_client'@'%';
FLUSH PRIVILEGES;

-- Also ensure root exists with native password just in case
CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY '';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%';
FLUSH PRIVILEGES;
