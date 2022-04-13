import { Canary } from "@flanksource/flanksource-ui/dist/components";
import { CanaryChecker } from "@flanksource/flanksource-ui/dist/api/axios";
import "./index.css";
function App() {
  CanaryChecker.defaults.baseURL = "/";
  return (
    <div className="max-w-screen-xl mx-auto flex justify-center">
      <Canary url="/api" />
    </div>
  );
}
export default App;