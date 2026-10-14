import { QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { queryClient } from './lib/queryClient';
import AppShell from './routes/AppShell';
import Login from './routes/Login';
import { RequireSession } from './lib/auth';
import Dashboard from './routes/cockpit/Dashboard';
import Calendar from './routes/cockpit/Calendar';
import Projects from './routes/cockpit/Projects';
import ProjectDetail from './routes/cockpit/ProjectDetail';
import Clients from './routes/cockpit/Clients';
import MyTasks from './routes/cockpit/MyTasks';
import Reports from './routes/cockpit/Reports';
import Workspace from './routes/cockpit/Workspace';
import Focus from './routes/cockpit/Focus';
import Remote from './routes/cockpit/Remote';
import Settings from './routes/cockpit/Settings';
import { useApplySkin } from './lib/skin';

export default function App() {
  useApplySkin();
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            element={
              <RequireSession>
                <AppShell />
              </RequireSession>
            }
          >
            <Route path="/" element={<Dashboard />} />
            <Route path="/hoje" element={<Navigate to="/" replace />} />
            <Route path="/calendario" element={<Calendar />} />
            <Route path="/projetos" element={<Projects />} />
            <Route path="/projetos/:id" element={<ProjectDetail />} />
            <Route path="/clientes" element={<Clients />} />
            <Route path="/minhas-tarefas" element={<MyTasks />} />
            <Route path="/relatorios" element={<Reports />} />
            <Route path="/workspace" element={<Workspace />} />
            <Route path="/foco" element={<Focus />} />
            <Route path="/remoto" element={<Remote />} />
            <Route path="/ajustes" element={<Settings />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
