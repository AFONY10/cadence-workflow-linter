public class Example {
    public static void main(String[] args) {
        // This call should be detected
        java.time.Instant.now();

        // Another example on a different line
        long t = System.currentTimeMillis();
        System.out.println("time: " + t);

        // A helper call to show multiple occurrences
        helper();
    }

    static void helper() {
        // detection here as well
        java.time.Instant.now();
    }
}
