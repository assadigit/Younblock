# Keep the gomobile bridge intact (reflection-free but referenced from
# generated go.Seq code); nothing else needs rules at this milestone.
-keep class mobile.** { *; }
-keep class go.** { *; }
