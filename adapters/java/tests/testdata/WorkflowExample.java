import com.uber.cadence.workflow.WorkflowInterface;
import com.uber.cadence.workflow.WorkflowMethod;

@WorkflowInterface
public interface MyWorkflow {
    @WorkflowMethod
    void runWorkflow();
}

public class MyWorkflowImpl implements MyWorkflow {
    @WorkflowMethod
    public void runWorkflow() {
        Helper.helper();
    }
    
}
