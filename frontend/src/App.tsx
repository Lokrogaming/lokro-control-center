import { Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "./lib/auth";
import Layout from "./components/Layout";
import Login from "./pages/Login";
import Overview from "./pages/Overview";
import Uptime from "./pages/Uptime";
import Servers from "./pages/Servers";
import Users from "./pages/Users";
import Permissions from "./pages/Permissions";
import Profile from "./pages/Profile";
import NotFound from "./pages/NotFound";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return null;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Overview />} />
          <Route path="servers" element={<Servers />} />
          <Route path="servers/:id" element={<Servers />} />
          <Route path="users" element={<Users />} />
          <Route path="uptime" element={<Uptime />} />
          <Route path="permissions" element={<Permissions />} />
          <Route path="profile" element={<Profile />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
