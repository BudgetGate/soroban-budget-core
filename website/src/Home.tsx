import { useState, useRef } from 'react';

const Home = () => {
  const [demoRunning, setDemoRunning] = useState(false);
  const typeWriterRef = useRef<HTMLSpanElement>(null);
  const outputRef = useRef<HTMLDivElement>(null);

  const runDemo = () => {
    if (demoRunning) return;
    setDemoRunning(true);
    
    if (typeWriterRef.current) {
      typeWriterRef.current.innerHTML = '';
    }
    if (outputRef.current) {
      outputRef.current.style.display = 'none';
    }

    const text = 'soroban-budget init --network futurenet';
    let i = 0;

    const typeWriter = () => {
      if (i < text.length) {
        if (typeWriterRef.current) {
          typeWriterRef.current.innerHTML += text.charAt(i);
        }
        i++;
        setTimeout(typeWriter, 50);
      } else {
        setTimeout(() => {
          if (outputRef.current) {
            outputRef.current.style.display = 'block';
          }
          setDemoRunning(false);
        }, 400);
      }
    };
    
    setTimeout(typeWriter, 100);
  };

  return (
    <div className="container mx-auto px-4 mt-8">
      <section className="hero flex flex-col items-center text-center">
        <div className="hero-content">
          <h1 className="text-3xl md:text-5xl lg:text-6xl font-bold mb-4">Official Soroban<br/>Budget CLI Toolkit</h1>
        </div>
        <p className="text-base md:text-lg mb-8 max-w-2xl">
          The authoritative toolkit for monitoring, deploying, and managing budget resources across your Soroban smart contracts. Engineered for precision and reliability.
        </p>
        <div className="hero-btns flex flex-col md:flex-row gap-4 mb-12 w-full md:w-auto justify-center">
          <a href="#install" className="btn btn-primary w-full md:w-auto text-center">Install Toolkit</a>
          <button onClick={runDemo} disabled={demoRunning} className="btn btn-secondary w-full md:w-auto">
            Run Demo
          </button>
        </div>

        <div className="terminal w-full max-w-3xl mx-auto rounded-lg overflow-hidden text-left">
          <div className="terminal-header bg-gray-800 p-2 flex items-center text-sm">
            Terminal - BudgetGate System
          </div>
          <div className="terminal-body p-4 bg-gray-900 text-gray-300">
            <div>$ <span ref={typeWriterRef} id="typewriter"></span><span className="cursor">&nbsp;</span></div>
            <div ref={outputRef} id="terminal-output" style={{ display: 'none', marginTop: '1rem', color: '#a0a0a0' }}>
              <div style={{ color: '#27c93f' }}>[✓] Initialization verified.</div>
              <div>[i] Analyzing configuration for 3 active contracts...</div>
              <div style={{ color: '#ffbd2e' }}>[*] Budget optimization verified at 24% efficiency increase.</div>
            </div>
          </div>
        </div>
      </section>

      <section className="features grid grid-cols-1 md:grid-cols-3 gap-8 mt-16 mb-16">
        <div className="feature-card p-6 border rounded-lg">
          <div className="feature-icon text-4xl mb-4">⚖️</div>
          <h3 className="text-xl font-bold mb-2">Authoritative Performance</h3>
          <p>Engineered with Go for maximum stability and millisecond execution. The definitive standard for Soroban budgets.</p>
        </div>
        <div className="feature-card p-6 border rounded-lg">
          <div className="feature-icon text-4xl mb-4">🛡️</div>
          <h3 className="text-xl font-bold mb-2">Verified Deployments</h3>
          <p>Simulate and validate budgets rigorously. Ensure continuous execution without exhausting computational resources.</p>
        </div>
        <div className="feature-card p-6 border rounded-lg">
          <div className="feature-icon text-4xl mb-4">🏛️</div>
          <h3 className="text-xl font-bold mb-2">Comprehensive Analytics</h3>
          <p>Visualize consumption through a powerful, certified reporting engine. Maintain complete oversight of your contracts.</p>
        </div>
      </section>
    </div>
  );
};

export default Home;
