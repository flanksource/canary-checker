import { Canary } from "@flanksource/flanksource-ui/dist/components";
import "./index.css";

function App() {
  return (
    <div className="max-w-screen-xl mx-auto flex justify-center">
      <Canary url="/canary/api" />
    </div>
  );
}

export default App;
