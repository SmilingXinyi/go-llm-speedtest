import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// https://vite.dev/config/
export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: {
        // 开发时把 /api 转发到后端 echo 服务（默认 :8787），前端同源调用 /api。
        proxy: {
            '/api': {
                target: 'http://localhost:8787',
                changeOrigin: true
            }
        }
    }
});
