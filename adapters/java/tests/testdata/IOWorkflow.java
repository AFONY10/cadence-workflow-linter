import com.uber.cadence.workflow.WorkflowInterface;
import com.uber.cadence.workflow.WorkflowMethod;
import java.io.FileInputStream;
import java.nio.file.Files;
import java.nio.file.Paths;

@WorkflowInterface
public interface IOWorkflow {
    @WorkflowMethod
    void run();
}

public class IOWorkflowImpl implements IOWorkflow {
    @WorkflowMethod
    public void run() {
        readAllBytesExample();
        try {
            int value = new FileInputStream("input.txt").read();
        } catch (Exception e) {
            // swallow for test purposes
        }
    }

    private byte[] readAllBytesExample() {
        try {
            return Files.readAllBytes(Paths.get("input.txt"));
        } catch (Exception e) {
            return new byte[0];
        }
    }
}
