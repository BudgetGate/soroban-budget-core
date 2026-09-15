const Docs = () => {
  return (
    <div className="container mx-auto px-4 mt-8">
      <section id="docs" className="documentation mb-16 p-8 border rounded-lg bg-gray-900 text-gray-200 shadow-lg">
        <h2 className="text-3xl font-bold mb-6 pb-2 border-b border-gray-700 text-white">Official Documentation</h2>
        
        <div className="prose prose-invert max-w-none space-y-6">
          <div>
            <h3 className="text-2xl font-semibold mb-3 text-blue-400">Architecture & Features</h3>
            <ul className="list-disc pl-5 space-y-2">
              <li><strong className="text-white">Dynamic Simulation</strong>: Leverages Soroban RPC endpoints to dynamically trace transaction execution footprints.</li>
              <li><strong className="text-white">Regression Engine</strong>: Compares newly generated snapshots against predefined baseline JSON files.</li>
              <li><strong className="text-white">Threshold Alerts</strong>: Automatically detects and highlights resource spikes in <code>CPUInstructions</code>, <code>MemoryBytes</code>, <code>ReadBytes</code>, and <code>WriteBytes</code>.</li>
            </ul>
          </div>

          <div>
            <h3 className="text-2xl font-semibold mb-3 text-blue-400">Deep Technical Details</h3>
            
            <div className="mb-4">
              <h4 className="text-xl font-medium mb-2 text-gray-100">XDR Parsing</h4>
              <p>The Go CLI interacts with the Soroban network by encoding and decoding Stellar XDR (External Data Representation). It utilizes the stellar/go SDK to strictly parse Base64 encoded transaction envelopes and simulation results.</p>
              <p>When fetching simulation data via RPC, the <code>result.transactionData</code> and <code>result.events</code> are received as base64-encoded XDR strings. The parser converts these into native Go structs representing <code>TransactionMeta</code>, extracting precise metrics for CPU Instructions and Memory usage injected by the Soroban environment.</p>
            </div>

            <div className="mb-4">
              <h4 className="text-xl font-medium mb-2 text-gray-100">Threshold Calculations</h4>
              <p>Threshold calculations form the core of the Regression Engine. The engine takes two maps: <code>baseline</code> and <code>current</code>.</p>
              <p>For each resource (e.g., <code>CPUInstructions</code>), the algorithm calculates the delta: <code>Delta = Current - Baseline</code>.</p>
              <p>If <code>Delta &gt; 0</code>, it computes the percentage increase: <code>Increase(%) = (Delta / Baseline) * 100</code>.</p>
              <p>A regression is flagged if the <code>Increase(%)</code> exceeds a user-defined threshold limit (default is 5%). This provides a safeguard against unnoticed efficiency degradations in smart contract code before deployment.</p>
            </div>

            <div className="mb-4">
              <h4 className="text-xl font-medium mb-2 text-gray-100">Markdown Diff Formatting</h4>
              <p>When the tool detects a regression or a successful check, it auto-generates a Markdown formatted table report.</p>
              <p>It uses a built-in text templating engine to format rows aligned with standard GitHub Flavored Markdown (GFM) tables.</p>
              <p>Green checkmarks (✅) and Red crosses (❌) are dynamically inserted based on threshold validations. This makes the output instantly readable when piped to PR comments via GitHub Actions or GitLab CI.</p>
            </div>
          </div>

          <div>
            <h3 className="text-2xl font-semibold mb-3 text-blue-400">CLI Usage</h3>
            <p>The tool operates by parsing JSON configurations and simulating smart contract executions on the testnet.</p>
            
            <h4 className="text-xl font-medium mt-4 mb-2 text-gray-100">Flags</h4>
            <ul className="list-disc pl-5 space-y-2">
              <li><code>--baseline</code> <em>(required)</em>: Path to the baseline JSON snapshot file containing the historically acceptable resource costs.</li>
              <li><code>--fixture</code> <em>(optional)</em>: Path to the JSON fixture file defining the contract execution parameters. If provided, the tool will dynamically run a simulation.</li>
              <li><code>--new</code> <em>(optional)</em>: Path to a static JSON snapshot file to compare against the baseline. (Mutually exclusive with <code>--fixture</code>).</li>
              <li><code>--rpc-url</code> <em>(optional)</em>: The Soroban RPC endpoint to use during dynamic simulations. Defaults to <code>https://soroban-testnet.stellar.org</code>.</li>
            </ul>
          </div>
        </div>
      </section>
    </div>
  );
};

export default Docs;
