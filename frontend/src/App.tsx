import {Routes, Route, Navigate} from 'react-router';
import AppLayout from './layouts/AppLayout';
import ReviewPage from './pages/ReviewPage';
import ConfigPage from './pages/ConfigPage';
import BenchPage from './pages/BenchPage';

export default function App() {
    return (
        <Routes>
            <Route element={<AppLayout />}>
                <Route index element={<ReviewPage />} />
                <Route path="bench" element={<BenchPage />} />
                <Route path="config" element={<ConfigPage />} />
                <Route path="history" element={<Navigate to="/" replace />} />
                <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
        </Routes>
    );
}
