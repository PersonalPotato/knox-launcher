// Compile-time stub only (the real annotation ships in projectzomboid.jar). Not packaged.
package se.krka.kahlua.integration.annotations;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.METHOD)
public @interface LuaMethod {
    String name();
    boolean global();
}
