import com.uber.cadence.activity.ActivityInterface;
import com.uber.cadence.activity.ActivityMethod;

@ActivityInterface
public interface MyActivity {
    @ActivityMethod
    void doActivity();
}

public class MyActivityImpl implements MyActivity {
    @ActivityMethod
    public void doActivity() {
        // Intentional time usage inside activity - should NOT be reported for workflows
        long t = System.currentTimeMillis();
    }
}
