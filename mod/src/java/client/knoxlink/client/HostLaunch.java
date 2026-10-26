package knoxlink.client;

import java.io.File;
import java.io.PrintWriter;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

public final class HostLaunch {
    private HostLaunch() {}

    static void log(String s) {
        System.out.println("[KnoxLink] " + s);
    }

    public static String agentArgument(String original) {
        try {
            Path agent = findAgentJar();
            Path zb = findZombieBuddyJar();
            if (agent == null || zb == null) {
                log("helper jars not found, hosted server starts without KnoxLink agent");
                return original;
            }
            String cacheDir = cacheDir();
            int n = writeManifest(cacheDir);
            log("wrote Workshop manifest with " + n + " items");
            return "-javaagent:" + agent + "=zb=" + zb + "|cache=" + cacheDir;
        } catch (Throwable t) {
            log("could not prepare hosted server agent: " + t);
            return original;
        }
    }

    static String cacheDir() throws Exception {
        Class<?> zfs = Class.forName("zombie.ZomboidFileSystem");
        Object inst = zfs.getField("instance").get(null);
        return (String) zfs.getMethod("getCacheDir").invoke(inst);
    }

    // Lines of "<itemID>|<installTimeStamp>|<installFolder>" for every Workshop item Steam has installed.
    static int writeManifest(String cacheDir) throws Exception {
        Class<?> sw = Class.forName("zombie.core.znet.SteamWorkshop");
        Object inst = sw.getField("instance").get(null);
        String[] folders = (String[]) sw.getMethod("GetInstalledItemFolders").invoke(inst);
        Method idOf = sw.getMethod("getIDFromItemInstallFolder", String.class);
        Method ts = sw.getMethod("GetItemInstallTimeStamp", long.class);
        File out = new File(cacheDir + File.separator + "Lua" + File.separator + "KnoxLink" + File.separator + "server_manifest.txt");
        out.getParentFile().mkdirs();
        int n = 0;
        try (PrintWriter w = new PrintWriter(out, StandardCharsets.UTF_8)) {
            if (folders != null) {
                for (String folder : folders) {
                    String id = (String) idOf.invoke(inst, folder);
                    if (id == null || id.isEmpty()) continue;
                    long stamp = (Long) ts.invoke(inst, Long.parseLong(id));
                    w.println(id + "|" + stamp + "|" + folder);
                    n++;
                }
            }
        }
        return n;
    }

    static Path findAgentJar() {
        try {
            Path self = Paths.get(HostLaunch.class.getProtectionDomain().getCodeSource().getLocation().toURI());
            // .../media/java/client/KnoxLink.jar -> .../media/java/agent/KnoxLinkAgent.jar
            Path p = self.getParent().getParent().resolve("agent").resolve("KnoxLinkAgent.jar");
            if (Files.isRegularFile(p)) return p.toAbsolutePath();
        } catch (Throwable ignored) {
        }
        try {
            Class<?> cgi = Class.forName("zombie.gameStates.ChooseGameInfo");
            Object mod = cgi.getMethod("getModDetails", String.class).invoke(null, "KnoxLink");
            if (mod != null) {
                for (String fieldName : new String[] {"versionDir", "commonDir"}) {
                    Field f = mod.getClass().getField(fieldName);
                    Object dir = f.get(mod);
                    if (dir == null) continue;
                    Path p = Paths.get(dir.toString(), "media", "java", "agent", "KnoxLinkAgent.jar");
                    if (Files.isRegularFile(p)) return p.toAbsolutePath();
                }
            }
        } catch (Throwable ignored) {
        }
        return null;
    }

    static Path findZombieBuddyJar() {
        try {
            Class<?> zb = Class.forName("me.zed_0xff.zombie_buddy.Agent");
            Path p = Paths.get(zb.getProtectionDomain().getCodeSource().getLocation().toURI());
            if (Files.isRegularFile(p)) return p.toAbsolutePath();
        } catch (Throwable ignored) {
        }
        Path p = Paths.get(System.getProperty("user.dir"), "ZombieBuddy.jar");
        return Files.isRegularFile(p) ? p.toAbsolutePath() : null;
    }
}
