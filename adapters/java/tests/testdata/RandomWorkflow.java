import com.uber.cadence.workflow.WorkflowInterface;
import com.uber.cadence.workflow.WorkflowMethod;
import java.util.Random;
import java.util.concurrent.ThreadLocalRandom;

@WorkflowInterface
public interface RandomWorkflow {
    @WorkflowMethod
    void run();
}

public class RandomWorkflowImpl implements RandomWorkflow {
    @WorkflowMethod
    public void run() {
        int a = new Random().nextInt();
        int b = ThreadLocalRandom.current().nextInt();
        RandomHelper.doRandom();
    }
}

class RandomHelper {
    static int doRandom() {
        Random rand = new Random();
        return rand.nextInt(10);
    }
}
