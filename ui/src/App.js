import { Badge, Icon, Toggle, Button, Canary } from "@flanksource/flanksource-ui/dist/components"
import './index.css'
function App() {

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div className="max-w-3xl mx-auto">
        <Canary url="/api" />
      </div>
    </div>
  );
}

export default App;
