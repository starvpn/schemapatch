-- 生产环境初始化脚本（示例）
-- 模拟旧的表结构

USE app_prod;

-- 用户表 (缺少avatar_url字段)
CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `username` VARCHAR(50) NOT NULL,
    `email` VARCHAR(100) NOT NULL,
    `password_hash` VARCHAR(255) NOT NULL,
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=正常, 0=禁用',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`),
    UNIQUE KEY `uk_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 订单表 (amount是DECIMAL类型, 缺少remark字段)
CREATE TABLE IF NOT EXISTS `orders` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` BIGINT UNSIGNED NOT NULL,
    `order_no` VARCHAR(32) NOT NULL,
    `amount` DECIMAL(10,2) NOT NULL COMMENT '金额',
    `status` VARCHAR(20) NOT NULL DEFAULT 'pending',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单表';

-- 临时日志表 (将被删除)
CREATE TABLE IF NOT EXISTS `temp_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `message` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='临时日志表';

-- 插入一些测试数据
INSERT INTO `users` (`username`, `email`, `password_hash`) VALUES
('admin', 'admin@example.com', 'hash123'),
('user1', 'user1@example.com', 'hash456');

INSERT INTO `orders` (`user_id`, `order_no`, `amount`, `status`) VALUES
(1, 'ORD20240101001', 99.99, 'completed'),
(1, 'ORD20240101002', 199.50, 'pending'),
(2, 'ORD20240102001', 50.00, 'completed');
