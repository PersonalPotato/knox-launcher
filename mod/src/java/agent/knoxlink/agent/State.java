package knoxlink.agent;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class State {
    private State() {}

    static final Map<Long, String> folders = new LinkedHashMap<>();
    static final Map<Long, Long> stamps = new LinkedHashMap<>();

    public static void log(String s) {
        System.out.println("[KnoxLink] " + s);
    }

    static void loadManifest(String cacheDir) {
        if (cacheDir == null) return;
        File f = new File(cacheDir + File.separator + "Lua" + File.separator + "KnoxLink" + File.separator + "server_manifest.txt");
        try {
            for (String line : Files.readAllLines(f.toPath(), StandardCharsets.UTF_8)) {
                String[] p = line.split("\\|", 3);
                if (p.length != 3) continue;
                long id = Long.parseLong(p[0].trim());
                if (!new File(p[2]).isDirectory()) continue;
                folders.put(id, p[2]);
                stamps.put(id, Long.parseLong(p[1].trim()));
            }
            log("manifest has " + folders.size() + " local Workshop items");
        } catch (Throwable t) {
            log("no Workshop manifest (" + t + ")");
        }
    }

    // Exit advice of ZomboidFileSystem.getInstalledItemModsFolders(List<String>)
    public static void addFolders(List<String> out) {
        for (String folder : folders.values()) {
            File mods = new File(folder, "mods");
            if (!mods.isDirectory()) continue;
            String abs = mods.getAbsolutePath();
            if (!out.contains(abs)) out.add(abs);
        }
    }

    // Enter advice of GameServerWorkshopItems.Install(ArrayList<Long>). Returns true when every
    // requested item is already on disk, so the download step is skipped.
    public static boolean installFromLocal(ArrayList<Long> ids) {
        try {
            if (ids == null || ids.isEmpty()) return false;
            for (Long id : ids) {
                if (!folders.containsKey(id)) {
                    log("item " + id + " is not installed locally, letting the server download items");
                    return false;
                }
            }
            String[] installFolders = new String[ids.size()];
            long[] timeStamps = new long[ids.size()];
            for (int i = 0; i < ids.size(); i++) {
                installFolders[i] = folders.get(ids.get(i));
                timeStamps[i] = stamps.get(ids.get(i));
            }
            Class<?> gs = Class.forName("zombie.network.GameServer");
            gs.getField("workshopInstallFolders").set(null, installFolders);
            gs.getField("workshopTimeStamps").set(null, timeStamps);
            log("using " + ids.size() + " Workshop items from the host's Steam folder (no download)");
            return true;
        } catch (Throwable t) {
            log("local Workshop install failed, falling back to download: " + t);
            return false;
        }
    }
}
