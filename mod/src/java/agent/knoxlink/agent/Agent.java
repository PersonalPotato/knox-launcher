package knoxlink.agent;

import java.io.File;
import java.lang.instrument.Instrumentation;
import java.util.HashMap;
import java.util.Map;
import java.util.jar.JarFile;

// Loaded into the hosted (co-op) server by the KnoxLink client helper.
// 1. Starts ZombieBuddy inside the server so server-side Java mods run.
// 2. Makes the server use the host's already-downloaded Workshop folders instead of re-downloading.
public class Agent {
    public static void premain(String args, Instrumentation inst) {
        Map<String, String> opts = new HashMap<>();
        if (args != null) {
            for (String part : args.split("\\|")) {
                int eq = part.indexOf('=');
                if (eq > 0) opts.put(part.substring(0, eq), part.substring(eq + 1));
            }
        }
        State.log("agent starting");
        String zb = opts.get("zb");
        if (zb == null || !new File(zb).isFile()) {
            State.log("ZombieBuddy.jar not found, cannot patch the server");
            return;
        }
        try {
            inst.appendToSystemClassLoaderSearch(new JarFile(zb));
            Class<?> zbAgent = Class.forName("me.zed_0xff.zombie_buddy.Agent", true, ClassLoader.getSystemClassLoader());
            // tinyfd shows ZombieBuddy's approval prompts as normal Windows dialogs; the console
            // prompt would block forever because the server's stdin belongs to the game.
            zbAgent.getMethod("premain", String.class, Instrumentation.class).invoke(null, "policy=prompt,frontend=tinyfd", inst);
        } catch (Throwable t) {
            State.log("could not start ZombieBuddy: " + (t.getCause() != null ? t.getCause() : t));
        }
        State.loadManifest(opts.get("cache"));
        try {
            Patcher.install(inst);
        } catch (Throwable t) {
            State.log("could not install patches: " + t);
        }
    }
}
