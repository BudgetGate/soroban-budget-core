import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import Home from './Home';
import Docs from './Docs';
import './index.css';

function App() {
  return (
    <Router>
      <div className="container mx-auto px-4">
        <nav className="navbar flex flex-col md:flex-row justify-between items-center py-4">
          <div className="logo text-xl md:text-2xl mb-4 md:mb-0">
            <Link to="/">Budget<span>Gate</span></Link>
          </div>
          <div className="nav-links flex flex-col md:flex-row gap-4 items-center">
            <Link to="/docs">Documentation</Link>
            <a href="https://github.com/BudgetGate/soroban-budget-core" target="_blank" rel="noopener noreferrer">Repository</a>
            <a href="https://budgetgate-docs.vercel.app/" target="_blank" rel="noopener noreferrer" className="text-yellow-600 font-bold">External Docs</a>
          </div>
        </nav>

        <main>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/docs" element={<Docs />} />
          </Routes>
        </main>

        <footer className="text-center py-8 text-sm mt-8 border-t">
          &copy; 2026 Soroban BudgetGate Initiative. Released under the MIT License.
        </footer>
      </div>
    </Router>
  );
}

export default App;
